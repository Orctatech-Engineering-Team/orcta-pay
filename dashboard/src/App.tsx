import { useState } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Layout } from "./components/Layout";
import { SettingsModal } from "./components/SettingsModal";
import { AppsPage } from "./pages/Apps";
import { ChargesPage } from "./pages/Charges";
import { GatewaysPage } from "./pages/Gateways";
import { LedgerPage } from "./pages/Ledger";
import { PayoutsPage } from "./pages/Payouts";
import { WebhooksPage } from "./pages/Webhooks";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
    },
  },
});

export default function App() {
  const [settingsOpen, setSettingsOpen] = useState(false);
  return (
    <QueryClientProvider client={queryClient}>
      <Layout onOpenSettings={() => setSettingsOpen(true)}>
        <Routes>
          <Route path="/" element={<Navigate to="/charges" replace />} />
          <Route path="/charges" element={<ChargesPage />} />
          <Route path="/payouts" element={<PayoutsPage />} />
          <Route path="/ledger" element={<LedgerPage />} />
          <Route path="/gateways" element={<GatewaysPage />} />
          <Route path="/apps" element={<AppsPage />} />
          <Route path="/webhooks" element={<WebhooksPage />} />
          <Route path="*" element={<Navigate to="/charges" replace />} />
        </Routes>
        <SettingsModal open={settingsOpen} onClose={() => setSettingsOpen(false)} />
      </Layout>
    </QueryClientProvider>
  );
}
