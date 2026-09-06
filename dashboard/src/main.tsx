import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import { Toast } from "@base-ui/react/toast";
import { router } from "./router";
import { toastManager } from "./lib/toast";
import "./index.css";
import "./styles/baseui.css";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
    },
  },
});

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <Toast.Provider toastManager={toastManager}>
        <RouterProvider router={router} />
      </Toast.Provider>
    </QueryClientProvider>
  </React.StrictMode>,
);
