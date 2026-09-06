import { useState, useEffect } from "react";
import { Outlet, useRouterState, useNavigate, Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@base-ui/react/button";
import { Select } from "@base-ui/react/select";
import { Toast } from "@base-ui/react/toast";
import { checkHealth } from "../lib/api";
import { getApiKey, getBaseUrl, getProduct, setProduct, maskKey } from "../lib/config";
import { getTheme, toggleTheme, type Theme } from "../lib/theme";
import { SettingsModal } from "./SettingsModal";

const NAV = [
  { to: "/", label: "Overview", hint: "overall" },
  { to: "/charges", label: "Charges", hint: "payment_intents" },
  { to: "/payouts", label: "Payouts", hint: "batches" },
  { to: "/ledger", label: "Ledger", hint: "double-entry" },
  { to: "/gateways", label: "Gateways", hint: "health" },
  { to: "/webhooks", label: "Webhooks", hint: "inbox" },
  { to: "/apps", label: "Apps", hint: "keys" },
] as const;

export function Layout() {
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [scope, setScope] = useState<string>(() => {
    try {
      const v = getProduct();
      return v || "all";
    } catch {
      return "all";
    }
  });
  const [theme, setThemeState] = useState<Theme>(() => getTheme());
  const apiKey = getApiKey();
  const baseUrl = getBaseUrl();
  const navigate = useNavigate();
  const pathname = useRouterState({ select: (s) => s.location.pathname });

  useEffect(() => {
    if (scope === "all") {
      try {
        localStorage.removeItem("orcta_pay_product");
      } catch { /* ignore */ }
    } else {
      setProduct(scope);
    }
  }, [scope]);

  const { data: healthData } = useQuery({
    queryKey: ["health", baseUrl],
    queryFn: () => checkHealth(),
    refetchInterval: 15000,
    staleTime: 10_000,
    retry: 1,
  });

  const health = healthData?.state ?? "checking";
  const detail = healthData?.detail ?? "checking…";
  const healthLabel = health === "ok" ? "live" : health === "down" ? "offline" : "checking";
  const dotClass = health === "ok" ? "ok" : health === "down" ? "down" : "checking";
  const isSystemAdmin = scope === "all";
  const roleLabel = isSystemAdmin ? "System admin" : "Manager";
  const roleHint = isSystemAdmin ? "All apps" : scope;

  return (
    <div className="workbench">
      <aside className="rail" aria-label="Primary">
        <div className="rail-brand">
          <img src="/logo.svg" alt="Orcta" width={28} height={28} />
          <div className="rail-brand-text">
            <span className="rail-brand-title">Orcta Pay</span>
            <span className="rail-brand-sub">operator · {healthLabel}</span>
          </div>
          <span style={{ marginLeft: "auto" }} className={`dot ${dotClass}`} aria-hidden />
        </div>

        <div className="rail-scope">
          <span className="rail-scope-label">Scope</span>
          <Select.Root value={scope} onValueChange={(v: unknown) => setScope(v as string)}>
            <Select.Trigger className="select" style={{ width: "100%" as unknown as string }}>
              <Select.Value />
              <Select.Icon>▾</Select.Icon>
            </Select.Trigger>
            <Select.Portal>
              <Select.Positioner>
                <Select.Popup>
                  <Select.List>
                    <Select.Item value="all">All apps — overall</Select.Item>
                    <Select.Item value="orctago">orctago</Select.Item>
                    <Select.Item value="pos">pos</Select.Item>
                  </Select.List>
                </Select.Popup>
              </Select.Positioner>
            </Select.Portal>
          </Select.Root>
          <div className="row" style={{ gap: 6 }}>
            <span className="pill" style={{ fontSize: 10 }}>{roleLabel}</span>
            <span className="muted" style={{ fontSize: 11 }}>{roleHint} · per-app or overall stats</span>
          </div>
          <span className="muted" style={{ fontSize: 11 }}>
            Vault <code>secret/orcta/orcta-pay/keys/{"{product}"}</code>
          </span>
        </div>

        <nav className="rail-nav" aria-label="Sections">
          <span className="rail-nav-label">Operate</span>
          {NAV.map((n) => {
            const active = pathname === n.to || (n.to === "/" && pathname === "/");
            return (
              <Link
                key={n.to}
                to={n.to}
                onClick={(e) => {
                  e.preventDefault();
                  void navigate({ to: n.to });
                }}
                className={`rail-link ${active ? "active" : ""}`}
              >
                <span className="rail-link-dot" aria-hidden />
                <span style={{ flex: 1 }}>{n.label}</span>
                <span className="muted" style={{ fontSize: 11 }}>{n.hint}</span>
              </Link>
            );
          })}
        </nav>

        <div className="rail-meta">
          <div className="rail-health" title={detail}>
            <span className={`dot ${dotClass}`} />
            <div className="rail-health-text">
              <span style={{ fontSize: 12, fontWeight: 600, color: "var(--color-ink)" }}>API {healthLabel}</span>
              <span className="muted" style={{ fontSize: 11, overflowWrap: "anywhere" }}>{baseUrl} · {detail.slice(0, 80)}</span>
            </div>
          </div>
          <div className="row" style={{ gap: 6, flexWrap: "wrap" }}>
            <span className="badge" title={apiKey || "no key"}>{maskKey(apiKey)}</span>
            <Button className="btn ghost" style={{ padding: "6px 10px", fontSize: 12, flex: 1 }} onClick={() => setSettingsOpen(true)}>
              Settings
            </Button>
          </div>
          <div className="rail-foot">
            Orcta Pay dashboard — Workbench
            <br />
            <span className="muted">N3 rail · Ft1 minimal · <code>/v1</code> proxy in dev</span>
          </div>
        </div>
      </aside>

      <div className="work-main">
        <div className="work-header">
          <div>
            <h1 className="work-header-title">
              {NAV.find((n) => n.to === pathname)?.label ?? (pathname === "/" ? "Overview" : pathname.replace("/", ""))}
            </h1>
            <span className="work-header-sub">
              {isSystemAdmin ? "Overall — all Orcta apps" : `Per-app — ${scope}`} · <code>{baseUrl}</code> {health === "ok" ? "· green" : health === "down" ? "· red" : "· checking"}
            </span>
          </div>
          <span className="work-header-spacer" />
          <Button
            className="theme-toggle"
            onClick={() => setThemeState(toggleTheme())}
            aria-label={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"}
            title={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"}
          >
            {theme === "dark" ? (
              <svg className="theme-toggle-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
                <circle cx="12" cy="12" r="4" />
                <path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41" />
              </svg>
            ) : (
              <svg className="theme-toggle-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden>
                <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
              </svg>
            )}
            {theme === "dark" ? "Light" : "Dark"}
          </Button>
          <span className={`badge ${health === "ok" ? "ok" : health === "down" ? "down" : ""}`}>
            <span className={`dot ${dotClass}`} style={{ marginRight: 6 }} />
            {healthLabel}
          </span>
          <span className="muted" style={{ fontSize: 11, maxWidth: 220, overflowWrap: "anywhere" }}>
            Proxy <code>/v1</code> → <code>{baseUrl}</code> · Mock when unreachable
          </span>
        </div>

        <div className="config-banner">
          <span>
            <span className={`dot ${dotClass}`} /> API <code>{baseUrl}</code> — {healthLabel} ({detail})
          </span>
          <span className="muted">
            Scope <code>{scope}</code> · {roleLabel} {isSystemAdmin ? "sees overall + per-app" : "sees per-app"}
          </span>
          <span className="work-header-spacer" />
          <span className="muted">Keys in Vault — create apps at <Link to="/apps">/apps</Link></span>
        </div>

        <main className="main">
          <Outlet />
        </main>

        <footer style={{ padding: "12px 24px", borderTop: "1px solid var(--color-line)", background: "var(--color-surface)", display: "flex", gap: 12, flexWrap: "wrap", fontSize: 11, color: "var(--color-ink-faint)" }}>
          <span>© Orcta · Pay</span>
          <span>·</span>
          <span>Vault <code>secret/orcta/orcta-pay/keys/{"{product}"}</code></span>
          <span>·</span>
          <span>TS client <code>@orctatech/orcta-pay</code> — pnpm only</span>
          <span style={{ marginLeft: "auto" }}>Workbench · overall + per-app</span>
        </footer>
      </div>

      <SettingsModal open={settingsOpen} onClose={() => setSettingsOpen(false)} />
      <Toast.Viewport />
    </div>
  );
}
