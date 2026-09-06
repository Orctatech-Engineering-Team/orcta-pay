import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { getApiKey, getBaseUrl } from "../lib/config";
import { mockGateways, type GatewayHealthRow } from "../lib/mock";

export function GatewaysPage() {
  const { data, error, isFetching } = useQuery({
    queryKey: ["gateways"],
    queryFn: async () => {
      const baseUrl = getBaseUrl().replace(/\/+$/, "");
      const res = await fetch(`${baseUrl}/v1/gateways/health`, {
        headers: { Accept: "application/json", Authorization: `Bearer ${getApiKey()}` },
      });
      if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
      return (await res.json()) as GatewayHealthRow[];
    },
    staleTime: 30_000,
    retry: 1,
  });

  const rows = data ?? mockGateways;
  const showMockBanner = !!error;

  const byChannel = useMemo(() => {
    const m = new Map<string, typeof rows>();
    for (const r of rows) {
      const arr = m.get(r.channel) || [];
      arr.push(r);
      m.set(r.channel, arr);
    }
    for (const arr of m.values()) arr.sort((a, b) => a.rank - b.rank);
    return m;
  }, [rows]);

  const openCount = rows.filter((r) => r.circuit_state === "open").length;
  const eligible = rows.filter((r) => r.eligible).length;
  const avgSuccess = rows.length ? rows.reduce((s, r) => s + r.rolling_success_rate, 0) / rows.length : 0;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      <div className="card">
        <div className="row" style={{ justifyContent: "space-between" }}>
          <h2 style={{ margin: 0 }}>Gateway health</h2>
          <span className="muted" style={{ fontFamily: "var(--font-mono)", fontSize: 12 }}>{isFetching ? "fetching..." : error ? "mock" : "live"}</span>
        </div>
        {showMockBanner ? (
          <div style={{ marginTop: 10, background: "var(--color-warn-soft)", border: "1px solid var(--color-warn-line)", padding: "8px 10px", borderRadius: 8, fontSize: 12 }}>
            Live API unreachable, showing mock data
          </div>
        ) : null}
      </div>

      <div className="stats-row">
        <div className="stat">
          <span className="stat-label">Eligible gateways</span>
          <span className="stat-value">{eligible}/{rows.length}</span>
          <span className="stat-meta">{rows.length - eligible} ineligible (below floor or circuit open)</span>
        </div>
        <div className="stat">
          <span className="stat-label">Circuits open</span>
          <span className="stat-value">{openCount}</span>
          <span className="stat-meta">Half-open {rows.filter((r) => r.circuit_state === "half_open").length} · closed {rows.filter((r) => r.circuit_state === "closed").length}</span>
        </div>
        <div className="stat">
          <span className="stat-label">Avg success rate</span>
          <span className="stat-value">{(avgSuccess * 100).toFixed(1)}%</span>
          <span className="stat-meta">Rolling window, floor 95%</span>
        </div>
        <div className="stat">
          <span className="stat-label">Failover</span>
          <span className="stat-meta" style={{ marginTop: 4 }}>Sync errors fail over to next-ranked gateway.</span>
        </div>
      </div>

      {[...byChannel.entries()].map(([channel, chRows]) => (
        <div key={channel} className="card" style={{ padding: 0, overflow: "hidden" }}>
          <div style={{ padding: "10px 14px", borderBottom: "1px solid var(--color-line)", background: "var(--color-surface-soft)" }}>
            <strong style={{ fontSize: 13 }}>Channel: {channel}</strong>{" "}
            <span className="muted">, ranked gateways</span>
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
                {chRows.map((r) => (
                  <tr key={`${r.gateway}-${r.channel}`} style={{ opacity: r.eligible ? 1 : 0.55 }}>
                    <td><strong>#{r.rank === 99 ? "\u2014" : r.rank}</strong></td>
                    <td>{r.gateway}</td>
                    <td>{r.eligible ? "yes" : "no (below floor / circuit open)"}</td>
                    <td>
                      <span style={{ fontWeight: 600 }}>{(r.rolling_success_rate * 100).toFixed(1)}%</span>
                      <span className="muted"> · {r.rolling_success_rate < 0.95 ? "below floor" : "above floor"}</span>
                    </td>
                    <td>{r.p95_latency_ms} ms</td>
                    <td><span className={`pill ${r.circuit_state}`}>{r.circuit_state}</span></td>
                    <td className="mono">{r.cost_bps} bps{r.cost_fixed_pesewas ? ` + ${r.cost_fixed_pesewas}ps` : ""}</td>
                    <td className="muted">{r.open_since ? new Date(r.open_since).toLocaleString() : "\u2014"}</td>
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
          Consecutive failures open the circuit. Half-open probes after cooldown.
        </p>
      </div>
    </div>
  );
}
