import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Input } from "@base-ui/react/input";
import { Button } from "@base-ui/react/button";
import { makeClient } from "../lib/api";
import { getApiKey, getBaseUrl } from "../lib/config";
import { formatDate, formatGHS } from "../lib/format";
import { mockCharges, type ChargeRow } from "../lib/mock";

function useLiveCharges(params: { q: string; product: string; gateway: string; status: string }) {
  return useQuery({
    queryKey: ["charges", params],
    queryFn: async () => {
      const baseUrl = getBaseUrl().replace(/\/+$/, "");
      const url = new URL(`${baseUrl}/v1/charges`);
      if (params.product !== "all") url.searchParams.set("product", params.product);
      if (params.gateway !== "all") url.searchParams.set("gateway", params.gateway);
      if (params.status !== "all") url.searchParams.set("status", params.status);
      if (params.q) url.searchParams.set("q", params.q);
      const res = await fetch(url.toString(), {
        headers: { Accept: "application/json", Authorization: `Bearer ${getApiKey()}` },
      });
      if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
      const data = (await res.json()) as ChargeRow[];
      return data;
    },
    staleTime: 30_000,
    retry: 1,
  });
}

export function ChargesPage() {
  const [q, setQ] = useState("");
  const [product, setProduct] = useState("all");
  const [gateway, setGateway] = useState("all");
  const [status, setStatus] = useState("all");
  const [selected, setSelected] = useState<ChargeRow | null>(null);

  const liveQuery = useLiveCharges({ q, product, gateway, status });

  const mockFiltered = useMemo(() => {
    return mockCharges.filter((r) => {
      if (q && !r.ref.toLowerCase().includes(q.toLowerCase())) return false;
      if (product !== "all" && r.product !== product) return false;
      if (gateway !== "all" && r.gateway !== gateway) return false;
      if (status !== "all" && r.status !== status) return false;
      return true;
    });
  }, [q, product, gateway, status]);

  const rows = liveQuery.data ?? mockFiltered;
  const showMockBanner = !!liveQuery.error;
  const succeeded = rows.filter((r) => r.status === "succeeded").length;
  const pending = rows.filter((r) => r.status === "pending").length;
  const failed = rows.filter((r) => r.status === "failed").length;
  const volumeSucceeded = rows.filter((r) => r.status === "succeeded").reduce((s, r) => s + r.amount_pesewas, 0);
  const total = rows.length;
  const rate = total ? (succeeded / total) * 100 : 0;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      <div className="card">
        <div className="row" style={{ justifyContent: "space-between" }}>
          <h2 style={{ margin: 0 }}>Charges</h2>
          <span className="muted" style={{ fontFamily: "var(--font-mono)", fontSize: 12 }}>{liveQuery.isFetching ? "fetching..." : liveQuery.error ? "mock" : "live"}</span>
        </div>
        <div className="row" style={{ marginTop: 12 }}>
          <Input className="input" placeholder="Search by ref..." value={q} onChange={(e) => setQ(e.target.value)} style={{ flex: 1, minWidth: 220 }} />
          <select className="select" value={product} onChange={(e) => setProduct(e.target.value)}>
            <option value="all">All apps</option>
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
        {showMockBanner ? (
          <div style={{ marginTop: 10, background: "var(--color-warn-soft)", border: "1px solid var(--color-warn-line)", padding: "8px 10px", borderRadius: 8, fontSize: 12 }}>
            Live API unreachable, showing mock data
          </div>
        ) : null}
      </div>

      <div className="stats-row">
        <div className="stat">
          <span className="stat-label">Total in view</span>
          <span className="stat-value">{total}</span>
          <span className="stat-meta">{succeeded} succeeded, {pending} pending, {failed} failed</span>
        </div>
        <div className="stat">
          <span className="stat-label">Success rate</span>
          <span className="stat-value">{rate.toFixed(1)}%</span>
          <span className="stat-meta">Succeeded / total in current filter</span>
        </div>
        <div className="stat">
          <span className="stat-label">Volume, succeeded</span>
          <span className="stat-value">{formatGHS(volumeSucceeded)}</span>
          <span className="stat-meta">Sum of succeeded amounts</span>
        </div>
        <div className="stat">
          <span className="stat-label">Inquiry</span>
          <span className="stat-meta" style={{ marginTop: 4 }}>Click a row to fetch live status</span>
        </div>
      </div>

      <div className="card" style={{ padding: 0, overflow: "hidden" }}>
        <div style={{ overflowX: "auto" }}>
          <table>
            <thead>
              <tr>
                <th>ref</th>
                <th>app</th>
                <th>gateway</th>
                <th>amount</th>
                <th>status</th>
                <th>created_at</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => (
                <tr key={r.ref} onClick={() => setSelected(r)} style={{ cursor: "pointer" }}>
                  <td title={r.ref} style={{ maxWidth: 280, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{r.ref}</td>
                  <td>{r.product}</td>
                  <td>{r.gateway}</td>
                  <td style={{ fontWeight: 600 }}>{formatGHS(r.amount_pesewas)}</td>
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

      {selected ? <ChargeDetail row={selected} onClose={() => setSelected(null)} /> : null}
    </div>
  );
}

function ChargeDetail({ row, onClose }: { row: ChargeRow; onClose: () => void }) {
  const { data, error, isFetching, refetch } = useQuery({
    queryKey: ["chargeStatus", row.ref],
    queryFn: async () => {
      const client = makeClient();
      const { data: d, error: e } = await client.getChargeStatus(row.ref);
      if (e) throw e;
      return d;
    },
    staleTime: 30_000,
    retry: 1,
  });

  return (
    <div className="card" style={{ borderColor: "var(--color-accent-ring)" }}>
      <div className="row" style={{ justifyContent: "space-between" }}>
        <h2 style={{ margin: 0, overflowWrap: "anywhere" }}>Charge {row.ref.slice(0, 28)}...</h2>
        <Button className="btn ghost" onClick={onClose}>Close</Button>
      </div>

      <div className="grid2" style={{ marginTop: 12 }}>
        <div>
          <h3 style={{ fontSize: 13, margin: "0 0 6px", fontWeight: 600 }}>Intent</h3>
          <dl className="kv">
            <dt>ref</dt><dd>{row.ref}</dd>
            <dt>app</dt><dd>{row.product}</dd>
            <dt>gateway</dt><dd>{row.gateway}</dd>
            <dt>amount</dt><dd>{formatGHS(row.amount_pesewas)} ({row.amount_pesewas} pesewas)</dd>
            <dt>status</dt><dd><span className={`pill ${row.status}`}>{row.status}</span></dd>
            <dt>created_at</dt><dd>{formatDate(row.created_at)}</dd>
            <dt>external_ref</dt><dd>{row.external_ref || "-"}</dd>
          </dl>
        </div>
        <div>
          <h3 style={{ fontSize: 13, margin: "0 0 6px", fontWeight: 600 }}>GetChargeStatus, live</h3>
          <div className="row" style={{ marginBottom: 8 }}>
            <Button className="btn" onClick={() => void refetch()} disabled={isFetching}>{isFetching ? "Fetching..." : "Fetch via OrctaPay.getChargeStatus"}</Button>
            <span className="muted">Calls <code>GET /v1/charges/{"{ref}"}/status</code></span>
          </div>
          {error ? <pre style={{ background: "var(--color-bad-soft)", padding: 10, borderRadius: 8, fontSize: 12, whiteSpace: "pre-wrap", border: "1px solid var(--color-bad-line)", overflowWrap: "anywhere" }}>{String((error as Error).message || error)}</pre> : null}
          {data != null ? <pre style={{ background: "var(--color-surface-muted)", padding: 10, borderRadius: 8, fontSize: 12, overflow: "auto", border: "1px solid var(--color-line)" }}>{JSON.stringify(data, null, 2)}</pre> : null}
          {data == null && !error && !isFetching ? <p className="muted">No live response yet. If the API is down you&apos;ll see a typed <code>OrctaPayError</code>.</p> : null}
        </div>
      </div>

      <div style={{ marginTop: 12 }}>
        <h3 style={{ fontSize: 13, margin: "0 0 6px", fontWeight: 600 }}>Raw gateway event</h3>
        {row.gateway_event ? (
          <pre style={{ background: "var(--color-surface-muted)", padding: 10, borderRadius: 8, fontSize: 12, overflow: "auto", border: "1px solid var(--color-line)" }}>{JSON.stringify(row.gateway_event, null, 2)}</pre>
        ) : (
          <p className="muted">No raw event stored for this intent (pending or failed before dispatch).</p>
        )}
      </div>
    </div>
  );
}
