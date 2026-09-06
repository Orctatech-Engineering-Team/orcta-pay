import { useState } from "react";
import { Outlet, useRouterState, useNavigate } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import { Tabs } from "@base-ui/react/tabs";
import { Button } from "@base-ui/react/button";
import { Toast } from "@base-ui/react/toast";
import { checkHealth } from "../lib/api";
import { getApiKey, getBaseUrl, maskKey } from "../lib/config";
import { SettingsModal } from "./SettingsModal";

export function Layout() {
  const [settingsOpen, setSettingsOpen] = useState(false);
  const apiKey = getApiKey();
  const baseUrl = getBaseUrl();
  const navigate = useNavigate();
  const pathname = useRouterState({ select: (s) => s.location.pathname });

  const { data: healthData } = useQuery({
    queryKey: ["health", baseUrl],
    queryFn: () => checkHealth(),
    refetchInterval: 15000,
    staleTime: 10_000,
    retry: 1,
  });

  const health = healthData?.state ?? "checking";
  const detail = healthData?.detail ?? "checking…";
  const healthLabel = health === "ok" ? "reachable" : health === "down" ? "unreachable" : health;
  const dotClass = health === "ok" ? "ok" : health === "down" ? "down" : "checking";

  // Map pathname to tab value — default to /charges for unknown
  const tabValue = ["/charges", "/payouts", "/ledger", "/gateways", "/apps", "/webhooks"].includes(pathname)
    ? pathname
    : "/charges";

  return (
    <div className="app-shell">
      <header className="topbar">
        <h1>Orcta Pay</h1>
        <Tabs.Root value={tabValue} onValueChange={(v: unknown) => navigate({ to: v as string })}>
          <Tabs.List>
            <Tabs.Tab value="/charges">Charges</Tabs.Tab>
            <Tabs.Tab value="/payouts">Payouts</Tabs.Tab>
            <Tabs.Tab value="/ledger">Ledger</Tabs.Tab>
            <Tabs.Tab value="/gateways">Gateways</Tabs.Tab>
            <Tabs.Tab value="/apps">Apps</Tabs.Tab>
            <Tabs.Tab value="/webhooks">Webhooks</Tabs.Tab>
          </Tabs.List>
        </Tabs.Root>
        <div className="spacer" />
        <span className={`badge ${health === "ok" ? "ok" : health === "down" ? "down" : ""}`} title={detail}>
          <span className={`dot ${dotClass}`} style={{ marginRight: 6 }} />
          {healthLabel}
        </span>
        <span className="badge" title={apiKey || "no key"}>{maskKey(apiKey)}</span>
        <Button className="btn secondary" style={{ padding: "6px 10px" }} onClick={() => setSettingsOpen(true)}>
          Settings
        </Button>
      </header>

      <div className="config-banner">
        <span>
          <span className={`dot ${dotClass}`} /> API <code>{baseUrl}</code> — {health === "ok" ? "green" : health === "down" ? "red" : "checking"} ({detail})
        </span>
        <span className="muted">
          Proxy: <code>/v1</code> → <code>{baseUrl}</code> in dev (see vite.config.ts). Mock data when unreachable.
        </span>
        <span className="spacer" />
        <span className="muted">
          Per-product keys: Vault <code>secret/orcta/orcta-pay/keys/{"{product}"}</code>
        </span>
      </div>

      <main className="main">
        <Outlet />
      </main>

      <SettingsModal open={settingsOpen} onClose={() => setSettingsOpen(false)} />
      <Toast.Viewport />
    </div>
  );
}
