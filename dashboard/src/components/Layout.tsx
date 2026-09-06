import { NavLink } from "react-router-dom";
import { useEffect, useState } from "react";
import { checkHealth, type HealthState } from "../lib/api";
import { getApiKey, getBaseUrl, maskKey } from "../lib/config";

export function Layout({ children, onOpenSettings }: { children: React.ReactNode; onOpenSettings: () => void }) {
  const [health, setHealth] = useState<HealthState>("checking");
  const [detail, setDetail] = useState("checking…");
  const apiKey = getApiKey();
  const baseUrl = getBaseUrl();

  useEffect(() => {
    let cancelled = false;
    const run = async () => {
      setHealth("checking");
      const res = await checkHealth();
      if (!cancelled) {
        setHealth(res.state);
        setDetail(res.detail);
      }
    };
    void run();
    const id = setInterval(() => void run(), 15000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [baseUrl]);

  const healthLabel = health === "ok" ? "reachable" : health === "down" ? "unreachable" : health;
  const dotClass = health === "ok" ? "ok" : health === "down" ? "down" : "checking";

  return (
    <div className="app-shell">
      <header className="topbar">
        <h1>Orcta Pay</h1>
        <nav>
          <NavLink to="/charges" className={({ isActive }) => (isActive ? "active" : "")}>Charges</NavLink>
          <NavLink to="/payouts" className={({ isActive }) => (isActive ? "active" : "")}>Payouts</NavLink>
          <NavLink to="/ledger" className={({ isActive }) => (isActive ? "active" : "")}>Ledger</NavLink>
          <NavLink to="/gateways" className={({ isActive }) => (isActive ? "active" : "")}>Gateways</NavLink>
          <NavLink to="/webhooks" className={({ isActive }) => (isActive ? "active" : "")}>Webhooks</NavLink>
        </nav>
        <div className="spacer" />
        <span className={`badge ${health === "ok" ? "ok" : health === "down" ? "down" : ""}`} title={detail}>
          <span className={`dot ${dotClass}`} style={{ marginRight: 6 }} />
          {healthLabel}
        </span>
        <span className="badge" title={apiKey || "no key"}>{maskKey(apiKey)}</span>
        <button className="btn secondary" onClick={onOpenSettings} style={{ padding: "6px 10px" }}>Settings</button>
      </header>

      <div className="config-banner">
        <span>
          <span className={`dot ${dotClass}`} /> API{" "}
          <code>{baseUrl}</code> — {health === "ok" ? "green" : health === "down" ? "red" : "checking"} ({detail})
        </span>
        <span className="muted">Proxy: <code>/v1</code> → <code>{baseUrl}</code> in dev (see vite.config.ts). Mock data when unreachable.</span>
        <span className="spacer" />
        <span className="muted">Per-product keys: Vault <code>secret/orcta/orcta-pay/keys/{"{product}"}</code></span>
      </div>

      <main className="main">{children}</main>
    </div>
  );
}
