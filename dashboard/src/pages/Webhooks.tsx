import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Select } from "@base-ui/react/select";
import { getApiKey, getBaseUrl } from "../lib/config";
import { formatDate } from "../lib/format";
import { mockWebhooks, type WebhookRow } from "../lib/mock";

export function WebhooksPage() {
  const [gateway, setGateway] = useState("all");

  const { data, error, isFetching } = useQuery({
    queryKey: ["webhooks", { gateway }],
    queryFn: async () => {
      const baseUrl = getBaseUrl().replace(/\/+$/, "");
      const url = new URL(`${baseUrl}/v1/webhooks`);
      if (gateway !== "all") url.searchParams.set("gateway", gateway);
      const res = await fetch(url.toString(), {
        headers: { Accept: "application/json", Authorization: `Bearer ${getApiKey()}` },
      });
      if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
      return (await res.json()) as WebhookRow[];
    },
    staleTime: 30_000,
    retry: 1,
  });

  const source = data ?? mockWebhooks;
  const showMockBanner = !!error;

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
          <div>
            <h2 style={{ margin: 0, fontSize: 16 }}>Webhook inbox</h2>
            <p className="muted" style={{ margin: "4px 0 0" }}>
              <code>webhook_inbox</code> (<code>aggregator_event_id</code> UNIQUE). Payload is a trigger, not truth — handler verifies HMAC (constant-time), dedups on <code>aggregator_event_id</code>, acks 200 durably, then calls <code>GetTransactionStatus</code> before writing the ledger. TanStack Query <code>["webhooks",{`{gateway}`}]</code> → <code>GET /v1/webhooks</code> with mock fallback.
            </p>
          </div>
          <span className="muted" style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>{isFetching ? "fetching…" : error ? "mock" : "live"}</span>
        </div>
        <div className="row" style={{ marginTop: 12 }}>
          <Select.Root value={gateway} onValueChange={(v: unknown) => setGateway(v as string)}>
            <Select.Trigger className="select"><Select.Value /><Select.Icon>▾</Select.Icon></Select.Trigger>
            <Select.Portal><Select.Positioner><Select.Popup><Select.List>
              <Select.Item value="all">All gateways</Select.Item>
              <Select.Item value="hubtel">hubtel</Select.Item>
              <Select.Item value="paystack">paystack</Select.Item>
              <Select.Item value="moolre">moolre</Select.Item>
            </Select.List></Select.Popup></Select.Positioner></Select.Portal>
          </Select.Root>
          <span className="muted">Dedup: <code>UNIQUE (aggregator_event_id)</code> — {deduped} duplicated event(s) highlighted · {unprocessed} unprocessed</span>
        </div>
        {showMockBanner ? (
          <div style={{ marginTop: 10, background: "var(--color-warn-soft)", border: "1px solid var(--color-warn-line)", padding: "8px 10px", borderRadius: 8, fontSize: 12 }}>
            Live API unreachable — showing mock data
          </div>
        ) : null}
      </div>

      <div className="stats-row">
        <div className="stat">
          <span className="stat-label">Total in view</span>
          <span className="stat-value">{rows.length}</span>
          <span className="stat-meta">Filtered inbox — {source.length} all gateways</span>
        </div>
        <div className="stat">
          <span className="stat-label">Dedup groups</span>
          <span className="stat-value">{deduped}</span>
          <span className="stat-meta">Redelivery is expected, not anomalous — duplicates return 200 without reprocessing</span>
        </div>
        <div className="stat">
          <span className="stat-label">Unprocessed</span>
          <span className="stat-value">{unprocessed}</span>
          <span className="stat-meta">Null until async processing completes — backstop reconciliation poll covers never-arrived webhooks</span>
        </div>
        <div className="stat">
          <span className="stat-label">Verification</span>
          <span className="stat-meta" style={{ marginTop: 4 }}>Paystack <code>x-paystack-signature</code> HMAC SHA512 constant-time. Moolre has no webhook signature — dedup + <code>GetTransactionStatus</code> is the check. Absence is valid <code>pending</code>.</span>
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
                    <td className="muted">{r.processed_at ? formatDate(r.processed_at) : <span title="null until async processing completes">— null</span>}</td>
                    <td>
                      {isDup ? (
                        <span className="pill" style={{ background: isSecondDup ? "var(--color-warn-soft)" : "var(--color-surface-muted)" }}>
                          {isSecondDup ? "duplicate — 200 no reprocess" : "first — processed"}
                        </span>
                      ) : (
                        <span className="muted">—</span>
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
          <li>Moolre has no published webhook signature — dedup plus <code>GetTransactionStatus</code> is the check.</li>
          <li>Paystack: <code>x-paystack-signature</code> HMAC SHA512 over raw body, constant-time compare. Reject unsigned → logged 401.</li>
          <li>Absence within the expected window is valid <code>pending</code>, not an error. Backstop reconciliation poll covers never-arrived webhooks.</li>
        </ul>
      </div>
    </div>
  );
}
