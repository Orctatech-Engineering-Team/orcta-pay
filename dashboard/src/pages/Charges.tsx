import { useEffect, useMemo, useState } from "react";
import { makeClient } from "../lib/api";
import { formatDate, formatGHS } from "../lib/format";
import { mockCharges, type ChargeRow } from "../lib/mock";

export function ChargesPage() {
  const [q, setQ] = useState("");
  const [product, setProduct] = useState("all");
  const [gateway, setGateway] = useState("all");
  const [status, setStatus] = useState("all");
  const [selected, setSelected] = useState<ChargeRow | null>(null);
  const [liveStatus, setLiveStatus] = useState<unknown>(null);
  const [liveError, setLiveError] = useState<string | null>(null);
  const [liveLoading, setLiveLoading] = useState(false);

  const rows = useMemo(() => {
    return mockCharges.filter((r) => {
      if (q && !r.ref.toLowerCase().includes(q.toLowerCase())) return false;
      if (product !== "all" && r.product !== product) return false;
      if (gateway !== "all" && r.gateway !== gateway) return false;
      if (status !== "all" && r.status !== status) return false;
      return true;
    });
  }, [q, product, gateway, status]);

  const open = (r: ChargeRow) => {
    setSelected(r);
    setLiveStatus(null);
    setLiveError(null);
  };

  return (
    <div>
      <div className="card">
        <h2>Charges — payment_intents</h2>
        <p className="muted">
          Thin view over <code>POST /v1/charges</code> / <code>GET /v1/charges/{"{ref}"}/status</code>. Mock data when API unreachable;
          detail calls live <code>GetChargeStatus</code> when <code>VITE_ORCTA_PAY_URL</code> is reachable.
        </p>
        <div className="row" style={{ marginTop: 10 }}>
          <input className="input" placeholder="Search by ref…" value={q} onChange={(e) => setQ(e.target.value)} style={{ flex: 1, minWidth: 220 }} />
          <select className="select" value={product} onChange={(e) => setProduct(e.target.value)}>
            <option value="all">All products</option>
            <option value="orctago">orctago</option>
            <option value="pos">pos</option>
          </select>
          <select className="select" value={gateway} onChange={(e) => setGateway(e.target.value)}>
            <option value="all">All gateways</option>
            <option value="hubtel">hubtel</option>
            <option value="paystack">paystack</option>
            <option value="moolre">moolre</option>
          </select>
          <select className="select" value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="all">All statuses</option>
            <option value="pending">pending</option>
            <option value="succeeded">succeeded</option>
            <option value="failed">failed</option>
          </select>
        </div>
      </div>

      <div className="card" style={{ padding: 0, overflow: "hidden" }}>
        <div style={{ overflowX: "auto" }}>
          <table>
            <thead>
              <tr>
                <th>ref</th>
                <th>product</th>
                <th>gateway</th>
                <th>amount</th>
                <th>status</th>
                <th>created_at</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => (
                <tr key={r.ref} onClick={() => open(r)} style={{ cursor: "pointer" }}>
                  <td className="mono" title={r.ref}>{r.ref}</td>
                  <td>{r.product}</td>
                  <td>{r.gateway}</td>
                  <td>{formatGHS(r.amount_pesewas)}</td>
                  <td><span className={`pill ${r.status}`}>{r.status}</span></td>
                  <td className="muted">{formatDate(r.created_at)}</td>
                </tr>
              ))}
              {rows.length === 0 && (
                <tr><td colSpan={6} className="muted" style={{ padding: 16 }}>No charges match filters.</td></tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {selected && (
        <ChargeDetail
          row={selected}
          liveStatus={liveStatus}
          liveError={liveError}
          liveLoading={liveLoading}
          onFetch={async () => {
            setLiveLoading(true);
            setLiveError(null);
            const client = makeClient();
            const { data, error } = await client.getChargeStatus(selected.ref);
            if (error) setLiveError(`${error.code} (${error.statusCode}): ${error.message}`);
            else setLiveStatus(data);
            setLiveLoading(false);
          }}
          onClose={() => setSelected(null)}
        />
      )}
    </div>
  );
}

function ChargeDetail({
  row,
  liveStatus,
  liveError,
  liveLoading,
  onFetch,
  onClose,
}: {
  row: ChargeRow;
  liveStatus: unknown;
  liveError: string | null;
  liveLoading: boolean;
  onFetch: () => void;
  onClose: () => void;
}) {
  useEffect(() => {
    // Auto-fetch once when opened to demonstrate wiring.
    void onFetch();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [row.ref]);

  return (
    <div className="card">
      <div className="row" style={{ justifyContent: "space-between" }}>
        <h2 style={{ margin: 0 }}>Charge detail — {row.ref.slice(0, 32)}…</h2>
        <button className="btn ghost" onClick={onClose}>Close</button>
      </div>

      <div className="grid2" style={{ marginTop: 12 }}>
        <div>
          <h3 style={{ fontSize: 13, margin: "0 0 6px" }}>Intent</h3>
          <dl className="kv">
            <dt>ref</dt><dd>{row.ref}</dd>
            <dt>product</dt><dd>{row.product}</dd>
            <dt>gateway</dt><dd>{row.gateway}</dd>
            <dt>amount</dt><dd>{formatGHS(row.amount_pesewas)} ({row.amount_pesewas} pesewas)</dd>
            <dt>status</dt><dd><span className={`pill ${row.status}`}>{row.status}</span></dd>
            <dt>created_at</dt><dd>{formatDate(row.created_at)}</dd>
            <dt>external_ref</dt><dd>{row.external_ref || "—"}</dd>
          </dl>
        </div>
        <div>
          <h3 style={{ fontSize: 13, margin: "0 0 6px" }}>GetChargeStatus (live)</h3>
          <div className="row" style={{ marginBottom: 8 }}>
            <button className="btn" onClick={onFetch} disabled={liveLoading}>{liveLoading ? "Fetching…" : "Fetch via OrctaPay.getChargeStatus"}</button>
            <span className="muted">Calls <code>GET /v1/charges/{"{ref}"}/status</code> through the TS client.</span>
          </div>
          {liveError ? <pre style={{ background: "#fef2f2", padding: 10, borderRadius: 8, fontSize: 12, whiteSpace: "pre-wrap" }}>{liveError}</pre> : null}
          {liveStatus != null ? <pre style={{ background: "#f1f5f9", padding: 10, borderRadius: 8, fontSize: 12, overflow: "auto" }}>{JSON.stringify(liveStatus, null, 2)}</pre> : null}
          {liveStatus == null && !liveError && !liveLoading ? <p className="muted">No live response yet. Click fetch — if the API is down you&apos;ll see a typed <code>OrctaPayError</code>.</p> : null}
        </div>
      </div>

      <div style={{ marginTop: 12 }}>
        <h3 style={{ fontSize: 13, margin: "0 0 6px" }}>Raw gateway event</h3>
        {row.gateway_event ? (
          <pre style={{ background: "#f1f5f9", padding: 10, borderRadius: 8, fontSize: 12, overflow: "auto" }}>{JSON.stringify(row.gateway_event, null, 2)}</pre>
        ) : (
          <p className="muted">No raw event stored for this intent (pending or failed before dispatch).</p>
        )}
      </div>
    </div>
  );
}
