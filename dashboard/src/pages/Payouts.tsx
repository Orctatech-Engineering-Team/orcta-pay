import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getApiKey, getBaseUrl } from "../lib/config";
import { formatDate, formatGHS, ageMinutes } from "../lib/format";
import { mockPayoutBatches, type PayoutBatchRow } from "../lib/mock";

export function PayoutsPage() {
  const [selected, setSelected] = useState<PayoutBatchRow | null>(null);

  const { data, error } = useQuery({
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

  return (
    <div>
      <div className="card">
        <h2>Payout batches</h2>
        <p className="muted">
          View over <code>payout_batches</code> + <code>payout_reservations</code>. Mock data when API unreachable. Click a batch for
          per-vendor lines and reservation age (open → settled / released).
        </p>
        {showMockBanner ? (
          <div style={{ marginTop: 10, background: "#fefce8", border: "1px solid #fde68a", padding: "8px 10px", borderRadius: 8, fontSize: 12 }}>
            Live API unreachable — showing mock data
          </div>
        ) : null}
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
        <div className="card">
          <div className="row" style={{ justifyContent: "space-between" }}>
            <h2 style={{ margin: 0 }}>Batch {selected.id} — {selected.batch_date}</h2>
            <button className="btn ghost" onClick={() => setSelected(null)}>Close</button>
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
                    <td className="muted">{l.reservation_resolved_at ? formatDate(l.reservation_resolved_at) : "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <p className="muted" style={{ marginTop: 10 }}>
            Reservation state machine: <code>open → settled</code> (disbursement confirmed, ledger posted) or{" "}
            <code>open → released</code> (permanent failure, funds return to available). An <code>open</code> past the threshold is a
            monitored alert, not a silent state. See <code>orcta-go-docs/PAYMENTS_SERVICE_DESIGN.md:5</code>.
          </p>
        </div>
      )}
    </div>
  );
}
