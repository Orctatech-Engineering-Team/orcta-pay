import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { makeClient } from "../lib/api";
import { formatDate } from "../lib/format";
import { mockApps, type AppRow } from "../lib/mock";

export function AppsPage() {
  const qc = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState("");
  const [product, setProduct] = useState("orctago");
  const [newKey, setNewKey] = useState<{ api_key: string; prefix: string; name: string } | null>(null);
  const [rotatedKey, setRotatedKey] = useState<{ api_key: string; id: string } | null>(null);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const { data, error, isLoading } = useQuery({
    queryKey: ["apps"],
    queryFn: async () => {
      const client = makeClient();
      const { data: d, error: e } = await client.listApps();
      if (e) throw e;
      // Normalize to AppRow shape; live App has same fields.
      return (d as unknown as AppRow[]) ?? [];
    },
    staleTime: 30_000,
    retry: 1,
  });

  const apps = data ?? mockApps;
  const showMockBanner = !!error;

  const createMut = useMutation({
    mutationFn: async (req: { name: string; product: string }) => {
      const client = makeClient();
      const { data: d, error: e } = await client.createApp(req);
      if (e) throw e;
      return d;
    },
    onSuccess: (d) => {
      if (d) {
        setNewKey({ api_key: d.api_key, prefix: d.prefix, name: d.name });
        setCreateOpen(false);
        setName("");
        void qc.invalidateQueries({ queryKey: ["apps"] });
      }
    },
    onError: (e: unknown) => setErrorMsg(e instanceof Error ? e.message : String(e)),
  });

  const rotateMut = useMutation({
    mutationFn: async (appId: string) => {
      const client = makeClient();
      const { data: d, error: e } = await client.rotateAppKey(appId);
      if (e) throw e;
      return { appId, data: d };
    },
    onSuccess: (res) => {
      if (res?.data) {
        setRotatedKey({ api_key: res.data.api_key, id: res.appId });
        void qc.invalidateQueries({ queryKey: ["apps"] });
      }
    },
    onError: (e: unknown) => setErrorMsg(e instanceof Error ? e.message : String(e)),
  });

  const revokeMut = useMutation({
    mutationFn: async (appId: string) => {
      const client = makeClient();
      const { error: e } = await client.revokeApp(appId);
      if (e) throw e;
      return appId;
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["apps"] }),
    onError: (e: unknown) => setErrorMsg(e instanceof Error ? e.message : String(e)),
  });

  const copy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      // fallback
      const el = document.createElement("textarea");
      el.value = text;
      document.body.appendChild(el);
      el.select();
      document.execCommand("copy");
      document.body.removeChild(el);
    }
  };

  return (
    <div>
      <div className="card">
        <div className="row" style={{ justifyContent: "space-between" }}>
          <div>
            <h2 style={{ margin: 0 }}>Apps — API keys</h2>
            <p className="muted" style={{ margin: "4px 0 0" }}>
              Each service creates an app and gets a per-product <code>pay_live_…</code> key. Keys are stored in Vault at{" "}
              <code>secret/orcta/orcta-pay/keys/{"{product}"}</code> and rendered to <code>ORCTA_PAY_API_KEY</code>.
            </p>
          </div>
          <button className="btn" onClick={() => { setCreateOpen(true); setErrorMsg(null); }}>Create app</button>
        </div>
        {showMockBanner ? (
          <div style={{ marginTop: 10, background: "#fefce8", border: "1px solid #fde68a", padding: "8px 10px", borderRadius: 8, fontSize: 12 }}>
            Live API unreachable — showing mock data
          </div>
        ) : null}
        {isLoading ? <p className="muted" style={{ marginTop: 8 }}>Loading…</p> : null}
        {errorMsg ? <pre style={{ background: "#fef2f2", padding: 10, borderRadius: 8, fontSize: 12, whiteSpace: "pre-wrap", marginTop: 10 }}>{errorMsg}</pre> : null}
      </div>

      {newKey ? (
        <div className="card" style={{ borderColor: "#f59e0b", background: "#fffbeb" }}>
          <h2 style={{ margin: 0 }}>API key — copy now, shown once</h2>
          <p className="muted">This key for <strong>{newKey.name}</strong> will not be shown again. Store it in Vault.</p>
          <div style={{ background: "#0f172a", color: "#e2e8f0", padding: 10, borderRadius: 8, fontSize: 13, marginTop: 8, display: "flex", justifyContent: "space-between", alignItems: "center", gap: 10 }}>
            <code style={{ wordBreak: "break-all" }}>{newKey.api_key}</code>
            <button className="btn secondary" style={{ flexShrink: 0 }} onClick={() => void copy(newKey.api_key)}>Copy</button>
          </div>
          <p className="muted" style={{ marginTop: 6 }}>Prefix: <code>{newKey.prefix}</code></p>
          <div style={{ background: "#f1f5f9", padding: 10, borderRadius: 8, fontSize: 12, marginTop: 10 }}>
            <strong>Use it in your service:</strong>
            <pre style={{ margin: "6px 0 0", whiteSpace: "pre-wrap", wordBreak: "break-all" }}>{`pnpm add @orctatech/orcta-pay
ORCTA_PAY_API_KEY=${newKey.api_key}
# Vault: secret/orcta/orcta-pay/keys/${product}
import { OrctaPay } from "@orctatech/orcta-pay";
const pay = new OrctaPay({ apiKey: process.env.ORCTA_PAY_API_KEY! });`}</pre>
            <button className="btn ghost" style={{ marginTop: 8 }} onClick={() => setNewKey(null)}>Dismiss</button>
          </div>
        </div>
      ) : null}

      {rotatedKey ? (
        <div className="card" style={{ borderColor: "#0ea5e9", background: "#f0f9ff" }}>
          <h2 style={{ margin: 0 }}>Rotated key — copy now, shown once</h2>
          <div style={{ background: "#0f172a", color: "#e2e8f0", padding: 10, borderRadius: 8, fontSize: 13, marginTop: 8, display: "flex", justifyContent: "space-between", alignItems: "center", gap: 10 }}>
            <code style={{ wordBreak: "break-all" }}>{rotatedKey.api_key}</code>
            <button className="btn secondary" style={{ flexShrink: 0 }} onClick={() => void copy(rotatedKey.api_key)}>Copy</button>
          </div>
          <div style={{ background: "#f1f5f9", padding: 10, borderRadius: 8, fontSize: 12, marginTop: 10 }}>
            <pre style={{ margin: 0, whiteSpace: "pre-wrap", wordBreak: "break-all" }}>{`ORCTA_PAY_API_KEY=${rotatedKey.api_key}`}</pre>
          </div>
          <button className="btn ghost" style={{ marginTop: 8 }} onClick={() => setRotatedKey(null)}>Dismiss</button>
        </div>
      ) : null}

      <div className="card" style={{ padding: 0, overflow: "hidden" }}>
        <div style={{ overflowX: "auto" }}>
          <table>
            <thead>
              <tr>
                <th>name</th>
                <th>product</th>
                <th>prefix</th>
                <th>created_at</th>
                <th>last_used_at</th>
                <th>revoked</th>
                <th>actions</th>
              </tr>
            </thead>
            <tbody>
              {apps.map((a) => (
                <tr key={a.id} style={{ opacity: a.revoked ? 0.6 : 1 }}>
                  <td className="mono">{a.name}</td>
                  <td>{a.product}</td>
                  <td className="mono">{a.prefix}</td>
                  <td className="muted">{formatDate(a.created_at)}</td>
                  <td className="muted">{a.last_used_at ? formatDate(a.last_used_at) : "—"}</td>
                  <td>{a.revoked ? <span className="pill failed">revoked</span> : <span className="pill succeeded">active</span>}</td>
                  <td>
                    <div className="row" style={{ gap: 6 }}>
                      <button
                        className="btn ghost"
                        style={{ padding: "4px 8px", fontSize: 12 }}
                        disabled={a.revoked || rotateMut.isPending}
                        onClick={() => { setErrorMsg(null); void rotateMut.mutateAsync(a.id); }}
                      >
                        {rotateMut.isPending ? "Rotating…" : "Rotate"}
                      </button>
                      <button
                        className="btn ghost"
                        style={{ padding: "4px 8px", fontSize: 12 }}
                        disabled={a.revoked || revokeMut.isPending}
                        onClick={() => { if (confirm(`Revoke ${a.name}?`)) { setErrorMsg(null); void revokeMut.mutateAsync(a.id); } }}
                      >
                        Revoke
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
              {apps.length === 0 && <tr><td colSpan={7} className="muted" style={{ padding: 16 }}>No apps yet. Create one.</td></tr>}
            </tbody>
          </table>
        </div>
      </div>

      {createOpen ? (
        <div className="modal-backdrop" onClick={() => setCreateOpen(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <h2 style={{ marginTop: 0 }}>Create app</h2>
            <p className="muted" style={{ marginTop: 4 }}>
              An app is a per-service API key. Name it after the service (<code>orctago</code>, <code>pos</code>). The key is shown once.
            </p>
            <label style={{ display: "block", marginTop: 12, fontSize: 13, fontWeight: 600 }}>Name</label>
            <input className="input" style={{ width: "100%", marginTop: 6 }} placeholder="orctago" value={name} onChange={(e) => setName(e.target.value)} />
            <label style={{ display: "block", marginTop: 12, fontSize: 13, fontWeight: 600 }}>Product</label>
            <select className="select" style={{ width: "100%", marginTop: 6 }} value={product} onChange={(e) => setProduct(e.target.value)}>
              <option value="orctago">orctago</option>
              <option value="pos">pos</option>
            </select>
            <div className="row" style={{ marginTop: 14, justifyContent: "flex-end" }}>
              <button className="btn ghost" onClick={() => setCreateOpen(false)}>Cancel</button>
              <button
                className="btn"
                disabled={!name.trim() || createMut.isPending}
                onClick={() => void createMut.mutateAsync({ name: name.trim(), product })}
              >
                {createMut.isPending ? "Creating…" : "Create"}
              </button>
            </div>
            {createMut.isError ? (
              <pre style={{ background: "#fef2f2", padding: 8, borderRadius: 8, fontSize: 12, whiteSpace: "pre-wrap", marginTop: 10 }}>
                {String((createMut.error as Error).message || createMut.error)}
              </pre>
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  );
}
