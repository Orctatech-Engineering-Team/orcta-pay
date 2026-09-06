import { createRouter, createRootRoute, createRoute } from "@tanstack/react-router";
import { Layout } from "./components/Layout";
import { OverviewPage } from "./pages/Overview";
import { ChargesPage } from "./pages/Charges";
import { PayoutsPage } from "./pages/Payouts";
import { LedgerPage } from "./pages/Ledger";
import { GatewaysPage } from "./pages/Gateways";
import { AppsPage } from "./pages/Apps";
import { WebhooksPage } from "./pages/Webhooks";
import { SettingsPage } from "./pages/Settings";
import { NotFoundPage } from "./pages/NotFound";
import { AuthGate } from "./components/AuthGate";

const rootRoute = createRootRoute({
  component: () => <AuthGate><Layout /></AuthGate>,
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: OverviewPage,
});

const overviewRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/overview",
  component: OverviewPage,
});

const chargesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/charges",
  component: ChargesPage,
});

const payoutsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/payouts",
  component: PayoutsPage,
});

const ledgerRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/ledger",
  component: LedgerPage,
});

const gatewaysRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/gateways",
  component: GatewaysPage,
});

const appsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/apps",
  component: AppsPage,
});

const webhooksRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/webhooks",
  component: WebhooksPage,
});

const settingsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/settings",
  component: SettingsPage,
});

const notFoundRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/$splat",
  component: NotFoundPage,
});

const routeTree = rootRoute.addChildren([
  indexRoute,
  overviewRoute,
  chargesRoute,
  payoutsRoute,
  ledgerRoute,
  gatewaysRoute,
  appsRoute,
  webhooksRoute,
  settingsRoute,
  notFoundRoute,
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
