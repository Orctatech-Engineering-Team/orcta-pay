import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getBaseUrl } from "../lib/config";
import { formatDate } from "../lib/format";
import type { WebhookRow } from "../lib/types";

export function WebhooksPage() {
  const [gateway, setGateway] = useState("all");

  const { data, error, isFetching } = useQuery({
    queryKey: ["webhooks", { gateway }],
    queryFn: async () => {
      const baseUrl = getBaseUrl().replace(/\/+$/, "");
      const url = new URL(`${baseUrl}/v1/webhooks`);
      if (gateway !== "all") url.searchParams.set("gateway", gateway);
      const res = await fetch(url.toString(), {
        credentials: "include",
        headers: { Accept: "application/json" },
      });
      if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
      return (await res.json()) as WebhookRow[];
    },
    staleTime: 30_000,
    retry: 1,
  });

  const source = data ?? [];

  const rows = useMemo(() => {
    if (gateway === "all") return source;
    return source.filter((r) => r.gateway === gateway);
  }, [source, gateway]);

  const dupIds = useMemo(() => {
    const counts = new Map<string, number>();
    for (const r of source) counts.set(r.aggregator_event_id, (counts.get(r.aggregator_event_id) || 0) + 1);
    return new Set([...counts.entries()].filter(([, c]) => c > 1).map(([k]) => k));
  }, [source]);

  const deduped = dupIds.size;
  const unprocessed = source.filter((r) => !r.processed_at).length;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      <div className="card">
        <div className="row" style={{ justifyContent: "space-between" }}>
          <h2 style={{ margin: 0 }}>Webhooks</h2>
          <span className="muted" style={{ fontFamily: "var(--font-mono)", fontSize: 12 }}>{isFetching ? "fetching..." : error ? "error" : "live"}</span>
        </div>
        <div className="row" style={{ marginTop: 12 }}>
          <select className="select" value={gateway} onChange={(e) => setGateway(e.target.value)}>
            <option value="all">All gateways</option>
            <option value="hubtel">hubtel</option>
            <option value="paystack">paystack</option>
            <option value="moolre">moolre</option>
          </select>
          <span className="muted">Dedup: <code>UNIQUE (aggregator_event_id)</code>, {deduped} duplicated event(s) highlighted, {unprocessed} unprocessed</span>
        </div>
        {error ? (
          <div style={{ marginTop: 10, background: "var(--color-warn-soft)", border: "1px solid var(--color-warn-line)", padding: "8px 10px", borderRadius: 8, fontSize: 12 }}>
            Could not load webhook events from the API. No placeholder events are shown.
          </div>
        ) : null}
      </div>

      <div className="stats-row">
        <div className="stat">
          <span className="stat-label">Total in view</span>
          <span className="stat-value">{rows.length}</span>
          <span className="stat-meta">Filtered inbox, {source.length} all gateways</span>
        </div>
        <div className="stat">
          <span className="stat-label">Dedup groups</span>
          <span className="stat-value">{deduped}</span>
          <span className="stat-meta">Duplicates return 200 without reprocessing</span>
        </div>
        <div className="stat">
          <span className="stat-label">Unprocessed</span>
          <span className="stat-value">{unprocessed}</span>
          <span className="stat-meta">Null until processing completes</span>
        </div>
      </div>

      <div className="card" style={{ padding: 0, overflow: "hidden" }}>
        <div style={{ overflowX: "auto" }}>
          <table>
            <thead>
              <tr>
                <th>aggregator_event_id</th>
                <th>gateway</th>
                <th>kind</th>
                <th>received_at</th>
                <th>processed_at</th>
                <th>dedup</th>
                <th>payload</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => {
                const isDup = dupIds.has(r.aggregator_event_id);
                const isSecondDup = isDup && r.processed_at === null;
                return (
                  <tr key={r.id} style={{ background: isSecondDup ? "var(--color-warn-soft)" : undefined }}>
                    <td className="mono">{r.aggregator_event_id}</td>
                    <td>{r.gateway}</td>
                    <td className="mono">{r.kind}</td>
                    <td className="muted">{formatDate(r.received_at)}</td>
                    <td className="muted">{r.processed_at ? formatDate(r.processed_at) : <span title="null until async processing completes">{"\u2014"} null</span>}</td>
                    <td>
                      {isDup ? (
                        <span className="pill" style={{ background: isSecondDup ? "var(--color-warn-soft)" : "var(--color-surface-muted)" }}>
                          {isSecondDup ? "duplicate, 200 no reprocess" : "first, processed"}
                        </span>
                      ) : (
                        <span className="muted">{"\u2014"}</span>
                      )}
                    </td>
                    <td>
                      <details>
                        <summary className="mono" style={{ cursor: "pointer" }}>view</summary>
                        <pre style={{ background: "var(--color-surface-muted)", padding: 8, borderRadius: 8, fontSize: 11, maxWidth: 320, overflow: "auto", whiteSpace: "pre-wrap", border: "1px solid var(--color-line)" }}>{JSON.stringify(r.payload, null, 2)}</pre>
                      </details>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      <div className="card">
        <h2>Notes</h2>
        <ul style={{ fontSize: 13, color: "var(--color-ink-muted)", margin: "6px 0 0", paddingLeft: 18, overflowWrap: "anywhere" }}>
          <li>Moolre: no webhook signature. Dedup + status polling is the check.</li>
          <li>Paystack: <code>x-paystack-signature</code> HMAC SHA512, constant-time.</li>
          <li>Absence within the expected window is valid <code>pending</code>.</li>
        </ul>
      </div>
    </div>
  );
}
