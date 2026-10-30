import { lazy, Suspense } from "react";
import { Toaster } from "@/components/ui/toaster";
import { Toaster as Sonner } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { HelmetProvider } from "react-helmet-async";
import { ThemeProvider } from "@/components/theme/ThemeProvider";
import { AuthGate } from "@/components/common/AuthGate";
import { OvhCredsGate } from "@/components/common/OvhCredsGate";
import { ActiveAccountSync } from "@/components/common/ActiveAccountSync";
import { ErrorBoundary } from "@/components/common/ErrorBoundary";

// 首页同步加载;其余页面按路由拆包,首屏不再下载全部页面(原单包约 1.9 MB)
import Index from "./pages/Index";
const NotFound = lazy(() => import("./pages/NotFound"));
const ServersPage = lazy(() => import("./pages/ServersPage"));
const QueuePage = lazy(() => import("./pages/QueuePage"));
const HistoryPage = lazy(() => import("./pages/HistoryPage"));
const MonitorPage = lazy(() => import("./pages/MonitorPage"));
const VpsMonitorPage = lazy(() => import("./pages/VpsMonitorPage"));
const LogsPage = lazy(() => import("./pages/LogsPage"));
const SettingsPage = lazy(() => import("./pages/SettingsPage"));
const AccountPage = lazy(() => import("./pages/AccountPage"));
const ServerControlPage = lazy(() => import("./pages/ServerControlPage"));
const VpsControlPage = lazy(() => import("./pages/VpsControlPage"));
const TelegramOrderPage = lazy(() => import("./pages/TelegramOrderPage"));
const TicketsPage = lazy(() => import("./pages/TicketsPage"));

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
    <ThemeProvider attribute="class" defaultTheme="dark" enableSystem>
      <QueryClientProvider client={queryClient}>
        <TooltipProvider>
          <Toaster />
          <Sonner position="top-center" />
          <BrowserRouter
            future={{
              v7_startTransition: true,
              v7_relativeSplatPath: true,
            }}
          >
            <AuthGate>
              <OvhCredsGate>
                <ActiveAccountSync />
                <ErrorBoundary>
                  <Suspense fallback={null}>
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
                      <Route path="/tickets" element={<TicketsPage />} />
                      <Route path="/telegram-order" element={<TelegramOrderPage />} />
                      <Route path="*" element={<NotFound />} />
                    </Routes>
                  </Suspense>
                </ErrorBoundary>
              </OvhCredsGate>
            </AuthGate>
          </BrowserRouter>
        </TooltipProvider>
      </QueryClientProvider>
    </ThemeProvider>
  </HelmetProvider>
);

export default App;
