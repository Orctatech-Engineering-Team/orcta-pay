import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { Button } from "@base-ui/react/button";
import { getApiKey, getBaseUrl, getProduct } from "../lib/config";
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
  const scope = getProduct() || "all";
  const isOverall = !scope || scope === "all" || ((): boolean => { try { return !localStorage.getItem("orcta_pay_product"); } catch { return true; } })();

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
      const b = payoutsQ.data; // payouts not product-scoped in mock, show shared but mark per scope
      return {
        product: p,
        charges: statsForCharges(c),
        ledger: statsForLedger(l),
        payouts: statsForPayouts(b),
      };
    });
  }, [products, chargesQ.data, ledgerQ.data, payoutsQ.data]);

  const openCircuits = gatewaysQ.data.filter((g) => g.circuit_state === "open").length;
  const halfOpen = gatewaysQ.data.filter((g) => g.circuit_state === "half_open").length;
  const eligible = gatewaysQ.data.filter((g) => g.eligible).length;
  const appsActive = appsQ.data.filter((a) => !a.revoked).length;
  const appsRevoked = appsQ.data.filter((a) => a.revoked).length;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      {showMockBanner ? (
        <div style={{ background: "var(--color-warn-soft)", border: "1px solid var(--color-warn-line)", padding: "10px 14px", borderRadius: 8, fontSize: 13, display: "flex", alignItems: "center", gap: 8 }}>
          <span className={`dot ${chargesQ.error ? "down" : "checking"}`} />
          Live API unreachable, showing mock data
        </div>
      ) : null}

      <div className="stats-row">
        <div className="stat">
          <span className="stat-label">Charges</span>
          <span className="stat-value">{chargesOverall.total}</span>
          <span className="stat-meta">
            {chargesOverall.succeeded} succeeded, {chargesOverall.pending} pending, {chargesOverall.failed} failed
          </span>
        </div>
        <div className="stat">
          <span className="stat-label">Success rate</span>
          <span className="stat-value">{(chargesOverall.rate * 100).toFixed(1)}%</span>
          <span className="stat-meta">Volume {formatGHS(chargesOverall.volumeSucceeded)}</span>
        </div>
        <div className="stat">
          <span className="stat-label">Payouts</span>
          <span className="stat-value">{formatGHS(payoutsOverall.net)}</span>
          <span className="stat-meta">{payoutsOverall.count} batches, {payoutsOverall.completed} completed</span>
        </div>
        <div className="stat">
          <span className="stat-label">Gateways</span>
          <span className="stat-value">{eligible}/{gatewaysQ.data.length}</span>
          <span className="stat-meta">{openCircuits} circuit open</span>
        </div>
      </div>

      <div className="grid2">
        <div className="card">
          <h2>Per-app</h2>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(220px, 1fr))", gap: 10, marginTop: 10 }}>
            {perApp.map((a) => (
              <div key={a.product} style={{ border: "1px solid var(--color-line-faint)", borderRadius: 10, padding: "12px 14px", background: "var(--color-surface-soft)" }}>
                <div className="row" style={{ justifyContent: "space-between", marginBottom: 8 }}>
                  <strong style={{ fontSize: 14 }}>{a.product}</strong>
                  <span className="pill" style={{ fontSize: 11 }}>{a.charges.total}</span>
                </div>
                <div style={{ fontSize: 13, display: "flex", flexDirection: "column", gap: 3, color: "var(--color-ink-muted)" }}>
                  <span>{a.charges.succeeded} succeeded, {a.charges.pending} pending, {a.charges.failed} failed</span>
                  <span style={{ fontFamily: "var(--font-mono)" }}>Vol {formatGHS(a.charges.volumeSucceeded)}</span>
                  <span style={{ fontFamily: "var(--font-mono)" }}>Ledger {formatGHS(a.ledger.net)}</span>
                </div>
                <div className="row" style={{ marginTop: 10, gap: 6 }}>
                  <Link to="/charges"><Button className="btn ghost" style={{ padding: "4px 8px", fontSize: 12 }}>Charges</Button></Link>
                  <Link to="/ledger"><Button className="btn ghost" style={{ padding: "4px 8px", fontSize: 12 }}>Ledger</Button></Link>
                </div>
              </div>
            ))}
            {perApp.length === 0 ? <span className="muted">No products yet.</span> : null}
          </div>
        </div>

        <div className="card">
          <h2>Apps</h2>
          <div style={{ display: "flex", gap: 12, marginTop: 10, marginBottom: 12 }}>
            <span className="pill succeeded">{appsActive} active</span>
            <span className="pill failed">{appsRevoked} revoked</span>
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            {appsQ.data.slice(0, 5).map((a) => (
              <div key={a.id} className="row" style={{ justifyContent: "space-between", padding: "6px 0", borderBottom: "1px solid var(--color-line-faint)" }}>
                <div className="row" style={{ gap: 8 }}>
                  <span className="mono" style={{ fontSize: 13 }}>{a.name}</span>
                  <span className="pill" style={{ fontSize: 11 }}>{a.product}</span>
                </div>
                <span className="muted" style={{ fontSize: 12 }}>{a.last_used_at ? formatDate(a.last_used_at) : "never"}</span>
              </div>
            ))}
          </div>
          <Link to="/apps"><Button className="btn" style={{ width: "100%", marginTop: 12 }}>Manage apps</Button></Link>
        </div>
      </div>

      <div className="grid2">
        <div className="card" style={{ padding: 0, overflow: "hidden" }}>
          <div style={{ padding: "10px 14px", borderBottom: "1px solid var(--color-line-faint)", background: "var(--color-surface-soft)" }}>
            <strong style={{ fontSize: 13 }}>Recent charges</strong>
          </div>
          <div style={{ overflowX: "auto" }}>
            <table>
              <thead><tr><th>ref</th><th>product</th><th>amount</th><th>status</th></tr></thead>
              <tbody>
                {chargesQ.data.slice(0, 5).map((r) => (
                  <tr key={r.ref}>
                    <td className="mono" title={r.ref} style={{ maxWidth: 200, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{r.ref}</td>
                    <td>{r.product}</td>
                    <td>{formatGHS(r.amount_pesewas)}</td>
                    <td><span className={`pill ${r.status}`}>{r.status}</span></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        <div className="card" style={{ padding: 0, overflow: "hidden" }}>
          <div style={{ padding: "10px 14px", borderBottom: "1px solid var(--color-line-faint)", background: "var(--color-surface-soft)" }}>
            <strong style={{ fontSize: 13 }}>Webhook inbox</strong>
          </div>
          <div style={{ overflowX: "auto" }}>
            <table>
              <thead><tr><th>event</th><th>gateway</th><th>received</th><th>status</th></tr></thead>
              <tbody>
                {webhooksQ.data.slice(0, 4).map((w) => (
                  <tr key={w.id}>
                    <td className="mono" style={{ maxWidth: 160, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{w.aggregator_event_id}</td>
                    <td>{w.gateway}</td>
                    <td className="muted">{formatDate(w.received_at)}</td>
                    <td><span className={`pill ${w.processed_at ? "succeeded" : "pending"}`}>{w.processed_at ? "done" : "pending"}</span></td>
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
