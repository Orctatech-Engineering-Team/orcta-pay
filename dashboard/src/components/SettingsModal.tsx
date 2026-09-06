import { useState } from "react";
import { getApiKey, getBaseUrl, getProduct, setApiKey, setBaseUrl, setProduct } from "../lib/config";

export function SettingsModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [apiKey, setApiKeyState] = useState(() => getApiKey());
  const [baseUrl, setBaseUrlState] = useState(() => getBaseUrl());
  const [product, setProductState] = useState(() => getProduct());

  if (!open) return null;

  const save = () => {
    setApiKey(apiKey.trim());
    setBaseUrl(baseUrl.trim());
    setProduct(product.trim().toLowerCase());
    onClose();
    // Reload so the client picks up new env without stale closure.
    window.location.reload();
  };

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h2 style={{ marginTop: 0 }}>Settings</h2>
        <p className="muted" style={{ marginTop: 4 }}>
          Paste any product&apos;s API key. Keys live in Vault at <code>secret/orcta/orcta-pay/keys/{"{product}"}</code> and are
          rendered to <code>.env</code> as <code>VITE_ORCTA_PAY_API_KEY</code>. This modal overrides via{" "}
          <code>localStorage</code> for demo — same shape any Orcta service uses with the TS client.
        </p>

        <label style={{ display: "block", marginTop: 12, fontSize: 13, fontWeight: 600 }}>API key (Bearer)</label>
        <input
          className="input"
          style={{ width: "100%", marginTop: 6 }}
          placeholder="pay_live_… or pay_test_…"
          value={apiKey}
          onChange={(e) => setApiKeyState(e.target.value)}
        />

        <label style={{ display: "block", marginTop: 12, fontSize: 13, fontWeight: 600 }}>Product</label>
        <input
          className="input"
          style={{ width: "100%", marginTop: 6 }}
          placeholder="orctago"
          value={product}
          onChange={(e) => setProductState(e.target.value)}
        />

        <label style={{ display: "block", marginTop: 12, fontSize: 13, fontWeight: 600 }}>Base URL</label>
        <input
          className="input"
          style={{ width: "100%", marginTop: 6 }}
          placeholder="http://localhost:8080"
          value={baseUrl}
          onChange={(e) => setBaseUrlState(e.target.value)}
        />

        <p className="muted" style={{ marginTop: 10 }}>
          Env fallback: <code>VITE_ORCTA_PAY_URL</code> and <code>VITE_ORCTA_PAY_API_KEY</code> in <code>dashboard/.env</code>.
          LocalStorage keys: <code>orcta_pay_api_key</code>, <code>orcta_pay_url</code>, <code>orcta_pay_product</code>.
        </p>

        <div className="row" style={{ marginTop: 14, justifyContent: "flex-end" }}>
          <button className="btn ghost" onClick={onClose}>Cancel</button>
          <button className="btn" onClick={save}>Save &amp; reload</button>
        </div>

        <div style={{ marginTop: 12, padding: 10, background: "#f1f5f9", borderRadius: 8, fontSize: 12, color: "#334155" }}>
          <strong>Any Orcta service can use the TS client the same way:</strong>
          <pre style={{ margin: "6px 0 0", whiteSpace: "pre-wrap", wordBreak: "break-all" }}>{`import { OrctaPay } from "@orctatech/orcta-pay";
const pay = new OrctaPay({ apiKey: process.env.VITE_ORCTA_PAY_API_KEY! });
// or: new OrctaPay({ apiKey: getApiKey() }) from localStorage`}</pre>
        </div>
      </div>
    </div>
  );
}
