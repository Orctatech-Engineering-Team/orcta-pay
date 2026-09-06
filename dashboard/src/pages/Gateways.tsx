import { mockGateways } from "../lib/mock";

export function GatewaysPage() {
  // Group by channel for ranking view.
  const byChannel = new Map<string, typeof mockGateways>();
  for (const r of mockGateways) {
    const arr = byChannel.get(r.channel) || [];
    arr.push(r);
    byChannel.set(r.channel, arr);
  }
  for (const arr of byChannel.values()) arr.sort((a, b) => a.rank - b.rank);

  return (
    <div>
      <div className="card">
        <h2>Gateway health — Valkey ranking</h2>
        <p className="muted">
          Rolling success rate, p95 latency, and circuit breaker per <code>gateway × channel</code> from Valkey. Ranking is{" "}
          <strong>success-rate floor → cost tiebreak</strong>; open circuits are excluded from ranking entirely. Source:{" "}
          <code>PAYMENTS_SERVICE_DESIGN.md:4</code>.
        </p>
        <div style={{ background: "#f1f5f9", padding: 10, borderRadius: 8, fontSize: 12, marginTop: 8 }}>
          <strong>Eligibility → Ranking → Failover:</strong> eligibility filters by channel/amount/currency/product; ranking scores
          remaining candidates; sync <code>Initiate</code> errors fail over to next-ranked gateway, async failures reconcile via{" "}
          <code>GetTransactionStatus</code> (no cross-gateway retry — avoids double-charge).
        </div>
      </div>

      {[...byChannel.entries()].map(([channel, rows]) => (
        <div key={channel} className="card" style={{ padding: 0, overflow: "hidden" }}>
          <div style={{ padding: "10px 14px", borderBottom: "1px solid #e2e8f0", background: "#f8fafc" }}>
            <strong style={{ fontSize: 13 }}>Channel: {channel}</strong>{" "}
            <span className="muted">— ranked gateways [primary, fallback…]</span>
          </div>
          <div style={{ overflowX: "auto" }}>
            <table>
              <thead>
                <tr>
                  <th>rank</th>
                  <th>gateway</th>
                  <th>eligible</th>
                  <th>success rate</th>
                  <th>p95 latency</th>
                  <th>circuit</th>
                  <th>cost</th>
                  <th>open_since</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((r) => (
                  <tr key={`${r.gateway}-${r.channel}`} style={{ opacity: r.eligible ? 1 : 0.55 }}>
                    <td><strong>#{r.rank === 99 ? "—" : r.rank}</strong></td>
                    <td>{r.gateway}</td>
                    <td>{r.eligible ? "yes" : "no (below floor / circuit open)"}</td>
                    <td>
                      <span style={{ fontWeight: 600 }}>{(r.rolling_success_rate * 100).toFixed(1)}%</span>
                      <span className="muted"> · {r.rolling_success_rate < 0.95 ? "below floor" : "above floor"}</span>
                    </td>
                    <td>{r.p95_latency_ms} ms</td>
                    <td><span className={`pill ${r.circuit_state}`}>{r.circuit_state}</span></td>
                    <td className="mono">{r.cost_bps} bps{r.cost_fixed_pesewas ? ` + ${r.cost_fixed_pesewas}ps` : ""}</td>
                    <td className="muted">{r.open_since ? new Date(r.open_since).toLocaleString() : "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      ))}

      <div className="card">
        <h2>Circuit breaker</h2>
        <p className="muted" style={{ margin: 0 }}>
          Consecutive failures / error-rate threshold opens the circuit. Half-open probes after cooldown. An outage is detected
          once and routed around for every subsequent request. Transitions are a release-blocking observability panel.
        </p>
      </div>
    </div>
  );
}
