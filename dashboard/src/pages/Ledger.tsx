import { useMemo, useState } from "react";
import { formatDate, formatGHS } from "../lib/format";
import { mockLedger, type LedgerEntry } from "../lib/mock";

function sumFor(entries: LedgerEntry[]): { credits: number; debits: number; net: number } {
  let credits = 0;
  let debits = 0;
  for (const e of entries) {
    if (e.entry_type === "credit") credits += e.amount_pesewas;
    else debits += e.amount_pesewas;
  }
  return { credits, debits, net: credits - debits };
}

export function LedgerPage() {
  const [vendor, setVendor] = useState("all");
  const [kind, setKind] = useState<"all" | "vendor" | "commission">("all");

  const filtered = useMemo(() => {
    return mockLedger.filter((e) => {
      if (vendor !== "all" && e.vendor_id !== vendor) return false;
      if (kind !== "all" && e.kind !== kind) return false;
      return true;
    });
  }, [vendor, kind]);

  const vendors = useMemo(() => [...new Set(mockLedger.map((e) => e.vendor_id))], []);
  const totals = sumFor(filtered);
  const reconciliationOk = totals.net === totals.credits - totals.debits;

  return (
    <div>
      <div className="card">
        <h2>Ledger — double-entry</h2>
        <p className="muted">
          <code>vendor_ledger_entries</code> + <code>platform_commission_entries</code> with{" "}
          <code>value_time</code> / <code>booking_time</code> / <code>settlement_time</code>. Filter by vendor. Reconciliation
          invariant per period: <code>sum(debits + credits) + sum(commission) + aggregator net = 0</code> — verified below over the
          visible slice.
        </p>
        <div className="row" style={{ marginTop: 10 }}>
          <select className="select" value={vendor} onChange={(e) => setVendor(e.target.value)}>
            <option value="all">All vendors</option>
            {vendors.map((v) => <option key={v} value={v}>{v}</option>)}
          </select>
          <select
            className="select"
            value={kind}
            onChange={(e) => setKind(e.target.value as typeof kind)}
          >
            <option value="all">All entry kinds</option>
            <option value="vendor">vendor</option>
            <option value="commission">commission</option>
          </select>
          <span className="muted">Showing {filtered.length} entries</span>
        </div>
      </div>

      <div className="card">
        <h2 style={{ marginBottom: 6 }}>Reconciliation — visible slice</h2>
        <div className="grid2">
          <div style={{ background: "#f1f5f9", padding: 10, borderRadius: 8, fontSize: 13 }}>
            <div>Credits: <strong>{formatGHS(totals.credits)}</strong> ({totals.credits} pesewas)</div>
            <div>Debits: <strong>{formatGHS(totals.debits)}</strong> ({totals.debits} pesewas)</div>
            <div>Net (credits − debits): <strong>{formatGHS(totals.net)}</strong></div>
          </div>
          <div style={{ background: reconciliationOk ? "#ecfdf5" : "#fef2f2", padding: 10, borderRadius: 8, fontSize: 13 }}>
            <div>Sum debits + credits + net = <strong>{reconciliationOk ? "0 ✓" : `${totals.credits - totals.debits - totals.net} ✗`}</strong></div>
            <div className="muted" style={{ marginTop: 4 }}>
              Per PAYMENTS_SERVICE_DESIGN: reconciliation job diffs this slice against aggregator settlement
              (Moolre <code>POST /open/account/status</code> type 2, Paystack <code>GET /settlement</code>). Mismatches alert, never
              auto-overwrite — correct with a linked compensating entry.
            </div>
          </div>
        </div>
      </div>

      <div className="card" style={{ padding: 0, overflow: "hidden" }}>
        <div style={{ overflowX: "auto" }}>
          <table>
            <thead>
              <tr>
                <th>id</th>
                <th>vendor</th>
                <th>kind</th>
                <th>type</th>
                <th>reason</th>
                <th>amount</th>
                <th>value_time</th>
                <th>booking_time</th>
                <th>settlement_time</th>
                <th>ref</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((e) => (
                <tr key={e.id}>
                  <td className="mono">{e.id}</td>
                  <td className="mono">{e.vendor_id}</td>
                  <td><span className="pill" style={{ textTransform: "none" }}>{e.kind}</span></td>
                  <td><span className={`pill ${e.entry_type === "credit" ? "succeeded" : "pending"}`}>{e.entry_type}</span></td>
                  <td className="mono">{e.reason}</td>
                  <td>{formatGHS(e.amount_pesewas)}</td>
                  <td className="muted">{formatDate(e.value_time)}</td>
                  <td className="muted">{formatDate(e.booking_time)}</td>
                  <td className="muted">{e.settlement_time ? formatDate(e.settlement_time) : <span title="null until Verify or reconciliation confirms">— null</span>}</td>
                  <td className="mono" style={{ maxWidth: 180, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={e.ref}>{e.ref}</td>
                </tr>
              ))}
              {filtered.length === 0 && <tr><td colSpan={10} className="muted" style={{ padding: 16 }}>No entries match filters.</td></tr>}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
