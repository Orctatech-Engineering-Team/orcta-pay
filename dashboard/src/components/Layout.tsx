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

function IconOverview() {
  return <svg className="rail-link-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round"><rect x="3" y="3" width="7" height="7" rx="1" /><rect x="14" y="3" width="7" height="7" rx="1" /><rect x="3" y="14" width="7" height="7" rx="1" /><rect x="14" y="14" width="7" height="7" rx="1" /></svg>;
}
function IconCharges() {
  return <svg className="rail-link-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round"><rect x="2" y="5" width="20" height="14" rx="2" /><line x1="2" y1="10" x2="22" y2="10" /></svg>;
}
function IconPayouts() {
  return <svg className="rail-link-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round"><line x1="12" y1="5" x2="12" y2="19" /><polyline points="19 12 12 19 5 12" /></svg>;
}
function IconLedger() {
  return <svg className="rail-link-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" /><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" /><line x1="8" y1="7" x2="16" y2="7" /><line x1="8" y1="11" x2="14" y2="11" /></svg>;
}
function IconGateways() {
  return <svg className="rail-link-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round"><circle cx="12" cy="12" r="3" /><path d="M12 2v4m0 12v4m10-10h-4M6 12H2m15.07-7.07l-2.83 2.83M9.76 14.24l-2.83 2.83m11.14 0l-2.83-2.83M9.76 9.76L6.93 6.93" /></svg>;
}
function IconWebhooks() {
  return <svg className="rail-link-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round"><path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z" /></svg>;
}
function IconApps() {
  return <svg className="rail-link-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" /><polyline points="3.27 6.96 12 12.01 20.73 6.96" /><line x1="12" y1="22.08" x2="12" y2="12" /></svg>;
}

const NAV_GROUPS = [
  {
    label: "Overview",
    items: [{ to: "/", label: "Overview", Icon: IconOverview }],
  },
  {
    label: "Money",
    items: [
      { to: "/charges", label: "Charges", Icon: IconCharges },
      { to: "/payouts", label: "Payouts", Icon: IconPayouts },
      { to: "/ledger", label: "Ledger", Icon: IconLedger },
    ],
  },
  {
    label: "Infrastructure",
    items: [
      { to: "/gateways", label: "Gateways", Icon: IconGateways },
      { to: "/webhooks", label: "Webhooks", Icon: IconWebhooks },
    ],
  },
  {
    label: "Access",
    items: [{ to: "/apps", label: "Apps", Icon: IconApps }],
  },
] ;

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

  return (
    <div className="workbench">
      <aside className="rail" aria-label="Primary">
        <div className="rail-brand">
          <img src="/logo.svg" alt="Orcta" width={28} height={28} />
          <div className="rail-brand-text">
            <span className="rail-brand-title">Orcta Pay</span>
            <span className="rail-brand-sub">operator</span>
          </div>
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
                    <Select.Item value="all">All apps (overall)</Select.Item>
                    <Select.Item value="orctago">orctago</Select.Item>
                    <Select.Item value="pos">pos</Select.Item>
                  </Select.List>
                </Select.Popup>
              </Select.Positioner>
            </Select.Portal>
          </Select.Root>
          <div className="row" style={{ gap: 6 }}>
            <span className="pill" style={{ fontSize: 11 }}>{roleLabel}</span>
          </div>
        </div>

        <nav className="rail-nav" aria-label="Sections">
          {NAV_GROUPS.map((group) => (
            <div key={group.label} className="rail-nav-group">
              <span className="rail-nav-label">{group.label}</span>
              {group.items.map((n) => {
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
                    <n.Icon />
                    <span>{n.label}</span>
                  </Link>
                );
              })}
            </div>
          ))}
        </nav>

        <div className="rail-meta">
          <div className="rail-health" title={detail}>
            <span className={`dot ${dotClass}`} />
            <div className="rail-health-text">
              <span style={{ fontSize: 12, fontWeight: 600, color: "var(--color-ink)" }}>API {healthLabel}</span>
              <span className="muted" style={{ fontSize: 12, overflowWrap: "anywhere" }}>{detail.slice(0, 60)}</span>
            </div>
          </div>
          <div className="row" style={{ gap: 6, flexWrap: "wrap" }}>
            <span className="badge" title={apiKey || "no key"}>{maskKey(apiKey)}</span>
            <Button className="btn ghost" style={{ padding: "6px 10px", fontSize: 12, flex: 1 }} onClick={() => setSettingsOpen(true)}>
              Settings
            </Button>
          </div>
        </div>
      </aside>

      <div className="work-main">
        <div className="work-header">
          <div>
            <h1 className="work-header-title">
              {NAV_GROUPS.flatMap((g) => g.items).find((n) => n.to === pathname)?.label ?? (pathname === "/" ? "Overview" : pathname.replace("/", ""))}
            </h1>
            <span className="work-header-sub">
              {isSystemAdmin ? "Overall, all Orcta apps" : `Per-app, ${scope}`}
            </span>
          </div>
          <span className="work-header-spacer" />
          <span className={`badge ${health === "ok" ? "ok" : health === "down" ? "down" : ""}`}>
            <span className={`dot ${dotClass}`} style={{ marginRight: 6 }} />
            {healthLabel}
          </span>
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
        </div>

        <main className="main">
          <Outlet />
        </main>

        <footer style={{ padding: "10px 24px", borderTop: "1px solid var(--color-line-faint)", background: "var(--color-surface)", display: "flex", gap: 12, flexWrap: "wrap", fontSize: 12, color: "var(--color-ink-faint)" }}>
          <span>© Orcta Pay</span>
          <span style={{ marginLeft: "auto" }}>TS client <code>@orctatech/orcta-pay</code></span>
        </footer>
      </div>

      <SettingsModal open={settingsOpen} onClose={() => setSettingsOpen(false)} />
      <Toast.Viewport />
    </div>
  );
}
