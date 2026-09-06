import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { getApiKey, getBaseUrl } from "../lib/config";
import { formatDate } from "../lib/format";
import { mockWebhooks, type WebhookRow } from "../lib/mock";

export function WebhooksPage() {
  const [gateway, setGateway] = useState("all");

  const { data, error } = useQuery({
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

  // Dedup: which aggregator_event_id appears more than once?
  const dupIds = useMemo(() => {
    const counts = new Map<string, number>();
    for (const r of source) counts.set(r.aggregator_event_id, (counts.get(r.aggregator_event_id) || 0) + 1);
    return new Set([...counts.entries()].filter(([, c]) => c > 1).map(([k]) => k));
  }, [source]);

  return (
    <div>
      <div className="card">
        <h2>Webhook inbox</h2>
        <p className="muted">
          <code>webhook_inbox</code> (<code>aggregator_event_id</code> UNIQUE). Payload is a trigger, not truth — handler verifies
          HMAC (constant-time), dedups on <code>aggregator_event_id</code>, acks 200 durably, then calls{" "}
          <code>GetTransactionStatus</code> for authoritative state before writing the ledger. Redelivery is expected, not
          anomalous — duplicates return 200 without reprocessing.
        </p>
        <div className="row" style={{ marginTop: 10 }}>
          <select className="select" value={gateway} onChange={(e) => setGateway(e.target.value)}>
            <option value="all">All gateways</option>
            <option value="hubtel">hubtel</option>
            <option value="paystack">paystack</option>
            <option value="moolre">moolre</option>
          </select>
          <span className="muted">Dedup: <code>UNIQUE (aggregator_event_id)</code> — highlighted rows share an event id.</span>
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
                  <tr key={r.id} style={{ background: isSecondDup ? "#fefce8" : undefined }}>
                    <td className="mono">{r.aggregator_event_id}</td>
                    <td>{r.gateway}</td>
                    <td className="mono">{r.kind}</td>
                    <td className="muted">{formatDate(r.received_at)}</td>
                    <td className="muted">{r.processed_at ? formatDate(r.processed_at) : <span title="null until async processing completes">— null</span>}</td>
                    <td>
                      {isDup ? (
                        <span className="pill" style={{ background: isSecondDup ? "#fef9c3" : "#f1f5f9" }}>
                          {isSecondDup ? "duplicate — 200 no reprocess" : "first — processed"}
                        </span>
                      ) : (
                        <span className="muted">—</span>
                      )}
                    </td>
                    <td>
                      <details>
                        <summary className="mono" style={{ cursor: "pointer" }}>view</summary>
                        <pre style={{ background: "#f1f5f9", padding: 8, borderRadius: 8, fontSize: 11, maxWidth: 320, overflow: "auto", whiteSpace: "pre-wrap" }}>{JSON.stringify(r.payload, null, 2)}</pre>
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
        <ul style={{ fontSize: 13, color: "#334155", margin: "6px 0 0", paddingLeft: 18 }}>
          <li>Moolre has no published webhook signature — dedup plus <code>GetTransactionStatus</code> is the check.</li>
          <li>Paystack: <code>x-paystack-signature</code> HMAC SHA512 over raw body, constant-time compare. Reject unsigned → logged 401.</li>
          <li>Absence within the expected window is valid <code>pending</code>, not an error. Backstop reconciliation poll covers never-arrived webhooks.</li>
        </ul>
      </div>
    </div>
  );
}
