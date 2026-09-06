import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@base-ui/react/button";
import { getApiKey, getBaseUrl } from "../lib/config";
import { formatDate, formatGHS, ageMinutes } from "../lib/format";
import { mockPayoutBatches, type PayoutBatchRow } from "../lib/mock";

export function PayoutsPage() {
  const [selected, setSelected] = useState<PayoutBatchRow | null>(null);

  const { data, error, isFetching } = useQuery({
    queryKey: ["payouts"],
    queryFn: async () => {
      const baseUrl = getBaseUrl().replace(/\/+$/, "");
      const res = await fetch(`${baseUrl}/v1/payouts`, {
        headers: { Accept: "application/json", Authorization: `Bearer ${getApiKey()}` },
      });
      if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
      return (await res.json()) as PayoutBatchRow[];
    },
    staleTime: 30_000,
    retry: 1,
  });

  const batches = data ?? mockPayoutBatches;
  const showMockBanner = !!error;
  const totals = useMemo(() => {
    const gross = batches.reduce((s, b) => s + b.gross_pesewas, 0);
    const net = batches.reduce((s, b) => s + b.net_pesewas, 0);
    const commission = batches.reduce((s, b) => s + b.commission_pesewas, 0);
    const vendors = batches.reduce((s, b) => s + b.vendor_count, 0);
    const completed = batches.filter((b) => b.status === "completed").length;
    const running = batches.filter((b) => b.status === "running").length;
    return { gross, net, commission, vendors, completed, running, count: batches.length };
  }, [batches]);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      <div className="card">
        <div className="row" style={{ justifyContent: "space-between" }}>
          <div>
            <h2 style={{ margin: 0, fontSize: 16 }}>Payout batches</h2>
            <p className="muted" style={{ margin: "4px 0 0" }}>
              View over <code>payout_batches</code> + <code>payout_reservations</code>. TanStack Query <code>["payouts"]</code> → <code>GET /v1/payouts</code>, fallback to mock. Click a batch for per-vendor lines and reservation age (open → settled / released).
            </p>
          </div>
          <span className="muted" style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>{isFetching ? "fetching…" : error ? "mock" : "live"}</span>
        </div>
        {showMockBanner ? (
          <div style={{ marginTop: 10, background: "var(--color-warn-soft)", border: "1px solid var(--color-warn-line)", padding: "8px 10px", borderRadius: 8, fontSize: 12 }}>
            Live API unreachable, showing mock data
          </div>
        ) : null}
      </div>

      <div className="stats-row">
        <div className="stat">
          <span className="stat-label">Batches in view</span>
          <span className="stat-value">{totals.count}</span>
          <span className="stat-meta">{totals.completed} completed · {totals.running} running · {totals.count - totals.completed - totals.running} other</span>
        </div>
        <div className="stat">
          <span className="stat-label">Gross</span>
          <span className="stat-value">{formatGHS(totals.gross)}</span>
          <span className="stat-meta">{totals.vendors} vendor lines · {formatGHS(totals.gross)} gross</span>
        </div>
        <div className="stat">
          <span className="stat-label">Net disbursed</span>
          <span className="stat-value">{formatGHS(totals.net)}</span>
          <span className="stat-meta">After commission {formatGHS(totals.commission)}</span>
        </div>
        <div className="stat">
          <span className="stat-label">Commission</span>
          <span className="stat-value">{formatGHS(totals.commission)}</span>
          <span className="stat-meta">Platform share, ledger kind commission</span>
        </div>
      </div>

      <div className="card" style={{ padding: 0, overflow: "hidden" }}>
        <div style={{ overflowX: "auto" }}>
          <table>
            <thead>
              <tr>
                <th>batch</th>
                <th>batch_date</th>
                <th>status</th>
                <th>vendors</th>
                <th>gross</th>
                <th>commission</th>
                <th>net</th>
                <th>created</th>
              </tr>
            </thead>
            <tbody>
              {batches.map((b) => (
                <tr key={b.id} onClick={() => setSelected(b)} style={{ cursor: "pointer" }}>
                  <td className="mono">{b.id}</td>
                  <td>{b.batch_date}</td>
                  <td><span className={`pill ${b.status}`}>{b.status}</span></td>
                  <td>{b.vendor_count}</td>
                  <td>{formatGHS(b.gross_pesewas)}</td>
                  <td>{formatGHS(b.commission_pesewas)}</td>
                  <td><strong>{formatGHS(b.net_pesewas)}</strong></td>
                  <td className="muted">{formatDate(b.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {selected && (
        <div className="card" style={{ borderColor: "var(--color-accent-ring)" }}>
          <div className="row" style={{ justifyContent: "space-between" }}>
            <h2 style={{ margin: 0, overflowWrap: "anywhere" }}>Batch {selected.id}, {selected.batch_date}</h2>
            <Button className="btn ghost" onClick={() => setSelected(null)}>Close</Button>
          </div>
          <p className="muted">
            Status <span className={`pill ${selected.status}`}>{selected.status}</span> · Gross {formatGHS(selected.gross_pesewas)} · Net {formatGHS(selected.net_pesewas)} · Created {formatDate(selected.created_at)}
            {selected.completed_at ? ` · Completed ${formatDate(selected.completed_at)}` : ""}
          </p>

          <div style={{ overflowX: "auto", marginTop: 10 }}>
            <table>
              <thead>
                <tr>
                  <th>vendor</th>
                  <th>recipient</th>
                  <th>gross</th>
                  <th>net</th>
                  <th>reservation</th>
                  <th>age</th>
                  <th>resolved</th>
                </tr>
              </thead>
              <tbody>
                {selected.lines.map((l) => (
                  <tr key={l.vendor_id}>
                    <td><span className="mono">{l.vendor_id}</span><br /><span className="muted">{l.vendor_name}</span></td>
                    <td className="mono">{l.recipient}</td>
                    <td>{formatGHS(l.amount_pesewas)}</td>
                    <td>{formatGHS(l.net_pesewas)}</td>
                    <td><span className={`pill ${l.reservation_status}`}>{l.reservation_status}</span></td>
                    <td className="muted">{ageMinutes(l.reservation_created_at)} ago</td>
                    <td className="muted">{l.reservation_resolved_at ? formatDate(l.reservation_resolved_at) : "\u2014"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <p className="muted" style={{ marginTop: 10 }}>
            Reservation: <code>open → settled</code> (disbursement confirmed, ledger posted) or <code>open → released</code> (permanent failure, funds return to available). <code>open</code> past threshold is a monitored alert, not silent state.
          </p>
        </div>
      )}
    </div>
  );
}
