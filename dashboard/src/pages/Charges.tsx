import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Input } from "@base-ui/react/input";
import { Button } from "@base-ui/react/button";
import { Select } from "@base-ui/react/select";
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

  return (
    <div>
      <div className="card">
        <h2>Charges — payment_intents</h2>
        <p className="muted">
          Thin view over <code>POST /v1/charges</code> / <code>GET /v1/charges/{"{ref}"}/status</code>. Mock data when API unreachable;
          detail calls live <code>GetChargeStatus</code> via TanStack Query when <code>VITE_ORCTA_PAY_URL</code> is reachable.
        </p>
        <div className="row" style={{ marginTop: 10 }}>
          <Input className="input" placeholder="Search by ref…" value={q} onChange={(e) => setQ(e.target.value)} style={{ flex: 1, minWidth: 220 }} />
          <Select.Root value={product} onValueChange={(v: unknown) => setProduct(v as string)}>
            <Select.Trigger className="select"><Select.Value /><Select.Icon>▾</Select.Icon></Select.Trigger>
            <Select.Portal><Select.Positioner><Select.Popup><Select.List>
              <Select.Item value="all">All products</Select.Item>
              <Select.Item value="orctago">orctago</Select.Item>
              <Select.Item value="pos">pos</Select.Item>
            </Select.List></Select.Popup></Select.Positioner></Select.Portal>
          </Select.Root>
          <Select.Root value={gateway} onValueChange={(v: unknown) => setGateway(v as string)}>
            <Select.Trigger className="select"><Select.Value /><Select.Icon>▾</Select.Icon></Select.Trigger>
            <Select.Portal><Select.Positioner><Select.Popup><Select.List>
              <Select.Item value="all">All gateways</Select.Item>
              <Select.Item value="hubtel">hubtel</Select.Item>
              <Select.Item value="paystack">paystack</Select.Item>
              <Select.Item value="moolre">moolre</Select.Item>
            </Select.List></Select.Popup></Select.Positioner></Select.Portal>
          </Select.Root>
          <Select.Root value={status} onValueChange={(v: unknown) => setStatus(v as string)}>
            <Select.Trigger className="select"><Select.Value /><Select.Icon>▾</Select.Icon></Select.Trigger>
            <Select.Portal><Select.Positioner><Select.Popup><Select.List>
              <Select.Item value="all">All statuses</Select.Item>
              <Select.Item value="pending">pending</Select.Item>
              <Select.Item value="succeeded">succeeded</Select.Item>
              <Select.Item value="failed">failed</Select.Item>
            </Select.List></Select.Popup></Select.Positioner></Select.Portal>
          </Select.Root>
        </div>
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
                <tr key={r.ref} onClick={() => setSelected(r)} style={{ cursor: "pointer" }}>
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
    <div className="card">
      <div className="row" style={{ justifyContent: "space-between" }}>
        <h2 style={{ margin: 0 }}>Charge detail — {row.ref.slice(0, 32)}…</h2>
        <Button className="btn ghost" onClick={onClose}>Close</Button>
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
            <Button className="btn" onClick={() => void refetch()} disabled={isFetching}>{isFetching ? "Fetching…" : "Fetch via OrctaPay.getChargeStatus"}</Button>
            <span className="muted">Calls <code>GET /v1/charges/{"{ref}"}/status</code> through the TS client.</span>
          </div>
          {error ? <pre style={{ background: "#fef2f2", padding: 10, borderRadius: 8, fontSize: 12, whiteSpace: "pre-wrap" }}>{String((error as Error).message || error)}</pre> : null}
          {data != null ? <pre style={{ background: "#f1f5f9", padding: 10, borderRadius: 8, fontSize: 12, overflow: "auto" }}>{JSON.stringify(data, null, 2)}</pre> : null}
          {data == null && !error && !isFetching ? <p className="muted">No live response yet. If the API is down you&apos;ll see a typed <code>OrctaPayError</code>.</p> : null}
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
