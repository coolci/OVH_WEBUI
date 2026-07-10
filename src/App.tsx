import { Toaster } from "@/components/ui/toaster";
import { Toaster as Sonner } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { HelmetProvider } from "react-helmet-async";
import { AuthGate } from "@/components/common/AuthGate";
import { OvhCredsGate } from "@/components/common/OvhCredsGate";
import { ActiveAccountSync } from "@/components/common/ActiveAccountSync";

import Index from "./pages/Index";
import NotFound from "./pages/NotFound";
import ServersPage from "./pages/ServersPage";
import QueuePage from "./pages/QueuePage";
import HistoryPage from "./pages/HistoryPage";
import MonitorPage from "./pages/MonitorPage";
import VpsMonitorPage from "./pages/VpsMonitorPage";
import LogsPage from "./pages/LogsPage";
import SettingsPage from "./pages/SettingsPage";
import AccountPage from "./pages/AccountPage";
import ServerControlPage from "./pages/ServerControlPage";
import VpsControlPage from "./pages/VpsControlPage";
import ContactChangePage from "./pages/ContactChangePage";
import PerformancePage from "./pages/PerformancePage";
import TelegramOrderPage from "./pages/TelegramOrderPage";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false,
      staleTime: 15_000,
    },
  },
});

const App = () => (
  <HelmetProvider>
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <Toaster />
        <Sonner />
        <BrowserRouter
          future={{
            v7_startTransition: true,
            v7_relativeSplatPath: true,
          }}
        >
          <AuthGate>
            <OvhCredsGate>
              <ActiveAccountSync />
              <Routes>
                <Route path="/" element={<Index />} />
                <Route path="/servers" element={<ServersPage />} />
                <Route path="/queue" element={<QueuePage />} />
                <Route path="/history" element={<HistoryPage />} />
                <Route path="/monitor" element={<MonitorPage />} />
                <Route path="/vps-monitor" element={<VpsMonitorPage />} />
                <Route path="/logs" element={<LogsPage />} />
                <Route path="/settings" element={<SettingsPage />} />
                <Route path="/account" element={<AccountPage />} />
                <Route path="/server-control" element={<ServerControlPage />} />
                <Route path="/vps-control" element={<VpsControlPage />} />
                <Route path="/contact-change" element={<ContactChangePage />} />
                <Route path="/performance" element={<PerformancePage />} />
                <Route path="/telegram-order" element={<TelegramOrderPage />} />
                <Route path="*" element={<NotFound />} />
              </Routes>
            </OvhCredsGate>
          </AuthGate>
        </BrowserRouter>
      </TooltipProvider>
    </QueryClientProvider>
  </HelmetProvider>
);

export default App;
