import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { Button } from "@base-ui/react/button";
import { getApiKey, getBaseUrl } from "../lib/config";
import { formatGHS, formatDate } from "../lib/format";
import { mockApps, mockCharges, mockGateways, mockLedger, mockPayoutBatches, mockWebhooks } from "../lib/mock";
import type { AppRow, ChargeRow, GatewayHealthRow, LedgerEntry, PayoutBatchRow, WebhookRow } from "../lib/mock";

function useLive<T>(key: string, path: string, fallback: T) {
  const q = useQuery({
    queryKey: [key],
    queryFn: async () => {
      const baseUrl = getBaseUrl().replace(/\/+$/, "");
      const res = await fetch(`${baseUrl}${path}`, {
        headers: { Accept: "application/json", Authorization: `Bearer ${getApiKey()}` },
      });
      if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
      return (await res.json()) as T;
    },
    staleTime: 30_000,
    retry: 1,
  });
  return { data: (q.data ?? fallback) as T, error: q.error, isLive: !q.error && !!q.data };
}

function statsForCharges(rows: ChargeRow[]) {
  const total = rows.length;
  const succeeded = rows.filter((r) => r.status === "succeeded").length;
  const pending = rows.filter((r) => r.status === "pending").length;
  const failed = rows.filter((r) => r.status === "failed").length;
  const volumeSucceeded = rows.filter((r) => r.status === "succeeded").reduce((s, r) => s + r.amount_pesewas, 0);
  const volumeAll = rows.reduce((s, r) => s + r.amount_pesewas, 0);
  const rate = total ? succeeded / total : 0;
  return { total, succeeded, pending, failed, volumeSucceeded, volumeAll, rate };
}

function statsForPayouts(batches: PayoutBatchRow[]) {
  const count = batches.length;
  const gross = batches.reduce((s, b) => s + b.gross_pesewas, 0);
  const net = batches.reduce((s, b) => s + b.net_pesewas, 0);
  const vendors = batches.reduce((s, b) => s + b.vendor_count, 0);
  const completed = batches.filter((b) => b.status === "completed").length;
  return { count, gross, net, vendors, completed };
}

function statsForLedger(entries: LedgerEntry[]) {
  let credits = 0;
  let debits = 0;
  for (const e of entries) {
    if (e.entry_type === "credit") credits += e.amount_pesewas;
    else debits += e.amount_pesewas;
  }
  return { credits, debits, net: credits - debits, count: entries.length };
}

export function OverviewPage() {
  const chargesQ = useLive<ChargeRow[]>("charges-overview", "/v1/charges", mockCharges);
  const payoutsQ = useLive<PayoutBatchRow[]>("payouts-overview", "/v1/payouts", mockPayoutBatches);
  const ledgerQ = useLive<LedgerEntry[]>("ledger-overview", "/v1/ledger", mockLedger);
  const gatewaysQ = useLive<GatewayHealthRow[]>("gateways-overview", "/v1/gateways/health", mockGateways);
  const appsQ = useLive<AppRow[]>("apps-overview", "/v1/apps", mockApps);
  const webhooksQ = useLive<WebhookRow[]>("webhooks-overview", "/v1/webhooks", mockWebhooks);

  const showMockBanner = !!(chargesQ.error || payoutsQ.error || ledgerQ.error || gatewaysQ.error || appsQ.error || webhooksQ.error);

  const chargesOverall = useMemo(() => statsForCharges(chargesQ.data), [chargesQ.data]);
  const payoutsOverall = useMemo(() => statsForPayouts(payoutsQ.data), [payoutsQ.data]);
  const ledgerOverall = useMemo(() => statsForLedger(ledgerQ.data), [ledgerQ.data]);

  const products = useMemo(() => [...new Set(chargesQ.data.map((c) => c.product))].sort(), [chargesQ.data]);
  const perApp = useMemo(() => {
    return products.map((p) => {
      const c = chargesQ.data.filter((r) => r.product === p);
      const l = ledgerQ.data.filter((e) => e.product === p);
      const b = payoutsQ.data;
      return {
        product: p,
        charges: statsForCharges(c),
        ledger: statsForLedger(l),
        payouts: statsForPayouts(b),
      };
    });
  }, [products, chargesQ.data, ledgerQ.data, payoutsQ.data]);

  const openCircuits = gatewaysQ.data.filter((g) => g.circuit_state === "open").length;
  const eligible = gatewaysQ.data.filter((g) => g.eligible).length;
  const appsActive = appsQ.data.filter((a) => !a.revoked).length;
  const appsRevoked = appsQ.data.filter((a) => a.revoked).length;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
      {showMockBanner ? (
        <div style={{ background: "var(--color-warn-soft)", border: "1px solid var(--color-warn-line)", padding: "10px 14px", borderRadius: 8, fontSize: 13, display: "flex", alignItems: "center", gap: 8 }}>
          <span className={`dot ${chargesQ.error ? "down" : "checking"}`} />
          Live API unreachable, showing mock data
        </div>
      ) : null}

      {/* Hero: Money processed */}
      <div className="card" style={{ background: "var(--color-accent-faint)", borderColor: "var(--color-accent-ring)" }}>
        <div style={{ display: "flex", alignItems: "baseline", gap: 12, flexWrap: "wrap" }}>
          <span style={{ fontSize: "var(--text-xs)", fontWeight: 600, letterSpacing: "var(--tracking-wide)", textTransform: "uppercase", color: "var(--color-accent-ink)" }}>Total processed</span>
        </div>
        <div style={{ fontSize: "var(--text-2xl)", fontWeight: 700, color: "var(--color-ink-strong)", marginTop: 4, letterSpacing: "var(--tracking-tight)" }}>
          {formatGHS(chargesOverall.volumeSucceeded)}
        </div>
        <div style={{ display: "flex", gap: 16, marginTop: 10, fontSize: "var(--text-sm)", color: "var(--color-ink-muted)" }}>
          <span>{chargesOverall.succeeded} succeeded</span>
          <span>{chargesOverall.pending} pending</span>
          <span>{chargesOverall.failed} failed</span>
          <span>Rate {(chargesOverall.rate * 100).toFixed(1)}%</span>
        </div>
      </div>

      {/* Secondary stats */}
      <div className="stats-row">
        <div className="stat">
          <span className="stat-label">Success rate</span>
          <span className="stat-value">{(chargesOverall.rate * 100).toFixed(1)}%</span>
          <span className="stat-meta">{chargesOverall.succeeded} of {chargesOverall.total} charges</span>
        </div>
        <div className="stat">
          <span className="stat-label">Payouts</span>
          <span className="stat-value">{formatGHS(payoutsOverall.net)}</span>
          <span className="stat-meta">{payoutsOverall.count} batches, {payoutsOverall.vendors} vendors</span>
        </div>
        <div className="stat">
          <span className="stat-label">Ledger balance</span>
          <span className="stat-value" style={{ color: ledgerOverall.net >= 0 ? "var(--color-ok-ink)" : "var(--color-bad-ink)" }}>{formatGHS(ledgerOverall.net)}</span>
          <span className="stat-meta">Credits {formatGHS(ledgerOverall.credits)} - Debits {formatGHS(ledgerOverall.debits)}</span>
        </div>
        <div className="stat">
          <span className="stat-label">Gateways</span>
          <span className="stat-value">{eligible}/{gatewaysQ.data.length}</span>
          <span className="stat-meta">{openCircuits} circuit open</span>
        </div>
      </div>

      {/* Per-app breakdown */}
      <div className="card">
        <h2 style={{ marginBottom: 14 }}>Per-app breakdown</h2>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))", gap: 12 }}>
          {perApp.map((a) => (
            <div key={a.product} style={{ border: "1px solid var(--color-line)", borderRadius: 10, padding: 16, background: "var(--color-surface)" }}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 10 }}>
                <strong style={{ fontSize: 15 }}>{a.product}</strong>
                <span className="pill succeeded">{a.charges.total} charges</span>
              </div>
              <div style={{ fontSize: 14, display: "flex", flexDirection: "column", gap: 6, color: "var(--color-ink)" }}>
                <div style={{ display: "flex", justifyContent: "space-between" }}>
                  <span>Volume</span>
                  <strong style={{ fontFamily: "var(--font-mono)" }}>{formatGHS(a.charges.volumeSucceeded)}</strong>
                </div>
                <div style={{ display: "flex", justifyContent: "space-between" }}>
                  <span>Ledger net</span>
                  <strong style={{ fontFamily: "var(--font-mono)", color: a.ledger.net >= 0 ? "var(--color-ok-ink)" : "var(--color-bad-ink)" }}>{formatGHS(a.ledger.net)}</strong>
                </div>
                <div style={{ display: "flex", justifyContent: "space-between", color: "var(--color-ink-muted)" }}>
                  <span>Success rate</span>
                  <span>{a.charges.total ? ((a.charges.succeeded / a.charges.total) * 100).toFixed(0) : 0}%</span>
                </div>
              </div>
              <div style={{ display: "flex", gap: 8, marginTop: 12, borderTop: "1px solid var(--color-line-faint)", paddingTop: 10 }}>
                <Link to="/charges"><Button className="btn ghost" style={{ padding: "4px 10px", fontSize: 13 }}>Charges</Button></Link>
                <Link to="/ledger"><Button className="btn ghost" style={{ padding: "4px 10px", fontSize: 13 }}>Ledger</Button></Link>
              </div>
            </div>
          ))}
          {perApp.length === 0 ? <span className="muted">No apps yet.</span> : null}
        </div>
      </div>

      {/* Apps and recent activity */}
      <div className="grid2">
        <div className="card">
          <h2 style={{ marginBottom: 10 }}>Apps</h2>
          <div style={{ display: "flex", gap: 12, marginBottom: 12 }}>
            <span className="pill succeeded">{appsActive} active</span>
            <span className="pill failed">{appsRevoked} revoked</span>
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            {appsQ.data.slice(0, 5).map((a) => (
              <div key={a.id} style={{ display: "flex", justifyContent: "space-between", padding: "8px 0", borderBottom: "1px solid var(--color-line-faint)", fontSize: 14 }}>
                <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                  <span className="mono" style={{ fontWeight: 500 }}>{a.name}</span>
                </div>
                <span className="muted" style={{ fontSize: 13 }}>{a.last_used_at ? formatDate(a.last_used_at) : "never"}</span>
              </div>
            ))}
          </div>
          <Link to="/apps"><Button className="btn" style={{ width: "100%", marginTop: 14 }}>Manage apps</Button></Link>
        </div>

        <div className="card" style={{ padding: 0, overflow: "hidden" }}>
          <div style={{ padding: "12px 16px", borderBottom: "1px solid var(--color-line-faint)", background: "var(--color-surface-soft)" }}>
            <strong style={{ fontSize: 14 }}>Recent charges</strong>
          </div>
          <div style={{ overflowX: "auto" }}>
            <table>
              <thead><tr><th>ref</th><th>app</th><th>amount</th><th>status</th></tr></thead>
              <tbody>
                {chargesQ.data.slice(0, 5).map((r) => (
                  <tr key={r.ref}>
                    <td className="mono" title={r.ref} style={{ maxWidth: 200, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{r.ref}</td>
                    <td>{r.product}</td>
                    <td style={{ fontFamily: "var(--font-mono)", fontWeight: 500 }}>{formatGHS(r.amount_pesewas)}</td>
                    <td><span className={`pill ${r.status}`}>{r.status}</span></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  );
}
