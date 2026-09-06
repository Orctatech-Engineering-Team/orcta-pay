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
      const b = payoutsQ.data; // payouts not product-scoped in mock — show shared but mark per scope
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
      <div className="card">
        <div className="row" style={{ justifyContent: "space-between" }}>
          <div>
            <h2 style={{ margin: 0, fontSize: 16 }}>{isOverall ? "Overall — all Orcta apps" : `Per-app — ${scope}`}</h2>
            <p className="muted" style={{ margin: "4px 0 0" }}>
              Manager sees per-app, System admin sees overall + per-app. Scope switcher in the rail controls filters everywhere.
              Data via <code>GET /v1/charges</code>, <code>/v1/payouts</code>, <code>/v1/ledger</code>, <code>/v1/gateways/health</code>, <code>/v1/webhooks</code>, <code>/v1/apps</code> — TanStack Query with mock fallback.
            </p>
          </div>
          <div className="row" style={{ gap: 8 }}>
            <Link to="/apps"><Button className="btn">Create app</Button></Link>
            <Link to="/charges"><Button className="btn ghost">View charges</Button></Link>
          </div>
        </div>
        {showMockBanner ? (
          <div style={{ marginTop: 12, background: "var(--color-warn-soft)", border: "1px solid var(--color-warn-line)", padding: "8px 10px", borderRadius: 8, fontSize: 12 }}>
            Live API unreachable — showing mock data
          </div>
        ) : null}
        <p className="muted" style={{ marginTop: 8 }}>
          Vault <code>secret/orcta/orcta-pay/keys/{"{product}"}</code> → <code>ORCTA_PAY_API_KEY</code>. New services create an app here and plug <code>pay_live_…</code> into the Go/TS SDKs.
        </p>
      </div>

      <div className="stats-row">
        <div className="stat">
          <span className="stat-label">Charges · overall</span>
          <span className="stat-value">{chargesOverall.total}</span>
          <span className="stat-meta">
            {chargesOverall.succeeded} succeeded · {chargesOverall.pending} pending · {chargesOverall.failed} failed · success rate {(chargesOverall.rate * 100).toFixed(1)}%
          </span>
          <span className="muted" style={{ fontFamily: "var(--font-mono)", fontSize: 12 }}>
            Volume (succeeded) {formatGHS(chargesOverall.volumeSucceeded)} · all {formatGHS(chargesOverall.volumeAll)}
          </span>
        </div>
        <div className="stat">
          <span className="stat-label">Payouts · overall</span>
          <span className="stat-value">{formatGHS(payoutsOverall.net)}</span>
          <span className="stat-meta">
            {payoutsOverall.count} batches · {payoutsOverall.completed} completed · {payoutsOverall.vendors} vendor lines
          </span>
          <span className="muted" style={{ fontFamily: "var(--font-mono)", fontSize: 12 }}>Gross {formatGHS(payoutsOverall.gross)} · net {formatGHS(payoutsOverall.net)}</span>
        </div>
        <div className="stat">
          <span className="stat-label">Ledger · balance (visible slice)</span>
          <span className="stat-value">{formatGHS(ledgerOverall.net)}</span>
          <span className="stat-meta">
            {ledgerOverall.count} entries · credits {formatGHS(ledgerOverall.credits)} · debits {formatGHS(ledgerOverall.debits)}
          </span>
          <span className="muted" style={{ fontSize: 11 }}>Reconciliation invariant checked on Ledger page</span>
        </div>
        <div className="stat">
          <span className="stat-label">Gateways · health</span>
          <span className="stat-value">{eligible}/{gatewaysQ.data.length} eligible</span>
          <span className="stat-meta">
            {openCircuits} open · {halfOpen} half-open · {gatewaysQ.data.length - openCircuits - halfOpen} closed
          </span>
          <span className="muted" style={{ fontSize: 11 }}>Ranking: success-rate floor → cost tiebreak · open excluded</span>
        </div>
      </div>

      <div className="grid2">
        <div className="card">
          <h2>Per-app cards</h2>
          <p className="muted">Each app’s charges, volume, and ledger derived from live or mock data. Overall aggregates above sum these.</p>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(240px, 1fr))", gap: 12, marginTop: 12 }}>
            {perApp.map((a) => (
              <div key={a.product} style={{ border: "1px solid var(--color-line)", borderRadius: 12, padding: 12, background: "var(--color-surface-soft)" }}>
                <div className="row" style={{ justifyContent: "space-between" }}>
                  <strong style={{ fontSize: 13 }}>{a.product}</strong>
                  <span className="pill">{a.charges.total} charges</span>
                </div>
                <div style={{ marginTop: 8, fontSize: 12, display: "flex", flexDirection: "column", gap: 4 }}>
                  <span>Succeeded {a.charges.succeeded} · Pending {a.charges.pending} · Failed {a.charges.failed} · rate {(a.charges.rate * 100).toFixed(1)}%</span>
                  <span style={{ fontFamily: "var(--font-mono)" }}>Vol succeeded {formatGHS(a.charges.volumeSucceeded)}</span>
                  <span style={{ fontFamily: "var(--font-mono)" }}>Ledger net {formatGHS(a.ledger.net)} · {a.ledger.count} entries</span>
                  <span className="muted" style={{ fontSize: 11 }}>Payouts shared across products in this slice — per-product payout split is a future ledger field.</span>
                </div>
                <div className="row" style={{ marginTop: 10, gap: 6 }}>
                  <Link to="/charges"><Button className="btn ghost" style={{ padding: "4px 8px", fontSize: 12 }}>Charges</Button></Link>
                  <Link to="/ledger"><Button className="btn ghost" style={{ padding: "4px 8px", fontSize: 12 }}>Ledger</Button></Link>
                </div>
              </div>
            ))}
            {perApp.length === 0 ? <span className="muted">No products in charge data — add a charge to see per-app split.</span> : null}
          </div>
        </div>

        <div className="card">
          <h2>Apps & keys · Vault</h2>
          <p className="muted">Per-product <code>pay_live_…</code> keys — create here, store in Vault, render to <code>ORCTA_PAY_API_KEY</code>.</p>
          <div style={{ marginTop: 12, display: "flex", flexDirection: "column", gap: 8 }}>
            <div className="row" style={{ gap: 8 }}>
              <span className="stat-label">Active</span><span className="pill succeeded">{appsActive}</span>
              <span className="stat-label">Revoked</span><span className="pill failed">{appsRevoked}</span>
              <span className="muted">{appsQ.data.length} total apps</span>
            </div>
            <div style={{ maxHeight: 220, overflow: "auto", border: "1px solid var(--color-line)", borderRadius: 8 }}>
              <table>
                <thead><tr><th>product</th><th>name</th><th>prefix</th><th>last used</th></tr></thead>
                <tbody>
                  {appsQ.data.slice(0, 6).map((a) => (
                    <tr key={a.id}>
                      <td>{a.product}</td>
                      <td className="mono">{a.name}</td>
                      <td className="mono">{a.prefix}</td>
                      <td className="muted">{a.last_used_at ? formatDate(a.last_used_at) : "—"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <Link to="/apps"><Button className="btn" style={{ width: "100%" }}>Create app — get pay_live_… now</Button></Link>
            <p className="muted" style={{ fontSize: 11 }}>Key shown once with copy + Vault path <code>secret/orcta/orcta-pay/keys/{"{product}"}</code> — same flow apps use via TS client <code>createApp</code> mutation with <code>invalidate ["apps"]</code>.</p>
          </div>
        </div>
      </div>

      <div className="grid2">
        <div className="card" style={{ padding: 0, overflow: "hidden" }}>
          <div style={{ padding: "10px 14px", borderBottom: "1px solid var(--color-line)", background: "var(--color-surface-soft)" }}>
            <strong style={{ fontSize: 13 }}>Recent charges</strong> <span className="muted">— latest 5 from {chargesQ.isLive ? "live" : "mock"}</span>
          </div>
          <div style={{ overflowX: "auto" }}>
            <table>
              <thead><tr><th>ref</th><th>product</th><th>amount</th><th>status</th></tr></thead>
              <tbody>
                {chargesQ.data.slice(0, 5).map((r) => (
                  <tr key={r.ref}>
                    <td className="mono" title={r.ref} style={{ maxWidth: 220, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{r.ref}</td>
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
          <div style={{ padding: "10px 14px", borderBottom: "1px solid var(--color-line)", background: "var(--color-surface-soft)" }}>
            <strong style={{ fontSize: 13 }}>Webhook inbox</strong> <span className="muted">— dedup via UNIQUE aggregator_event_id · latest 4</span>
          </div>
          <div style={{ overflowX: "auto" }}>
            <table>
              <thead><tr><th>event</th><th>gateway</th><th>received</th><th>processed</th></tr></thead>
              <tbody>
                {webhooksQ.data.slice(0, 4).map((w) => (
                  <tr key={w.id}>
                    <td className="mono">{w.aggregator_event_id}</td>
                    <td>{w.gateway}</td>
                    <td className="muted">{formatDate(w.received_at)}</td>
                    <td className="muted">{w.processed_at ? formatDate(w.processed_at) : "— null"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <div className="card" style={{ borderStyle: "dashed" }}>
        <h2>Metric to confirm</h2>
        <p className="muted">Stubs for metrics that need backend confirmation before showing deltas. No fabricated growth percentages.</p>
        <div className="grid2" style={{ marginTop: 10 }}>
          <div style={{ background: "var(--color-surface-muted)", border: "1px dashed var(--color-line-strong)", borderRadius: 8, padding: 12 }}>
            <span className="stat-label">Reconciliation lag — metric to confirm</span>
            <div style={{ height: 28, marginTop: 8, background: "var(--color-line)", borderRadius: 6, opacity: 0.5 }} />
            <span className="muted" style={{ fontSize: 11 }}>Will show value_time → settlement_time p50/p95 when ledger backfill ships.</span>
          </div>
          <div style={{ background: "var(--color-surface-muted)", border: "1px dashed var(--color-line-strong)", borderRadius: 8, padding: 12 }}>
            <span className="stat-label">Gateway cost saved — metric to confirm</span>
            <div style={{ height: 28, marginTop: 8, background: "var(--color-line)", borderRadius: 6, opacity: 0.5 }} />
            <span className="muted" style={{ fontSize: 11 }}>Ranking tiebreak savings not tracked yet — block stays grey until measured.</span>
          </div>
        </div>
      </div>
    </div>
  );
}
