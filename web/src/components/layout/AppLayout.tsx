import { ReactNode, useState, forwardRef } from "react";
import { AppSidebar } from "./AppSidebar";
import { TopBar } from "./TopBar";
import { StatusBar } from "./StatusBar";
import { MobileBottomNav } from "./MobileBottomNav";
import { CommandPalette } from "@/components/common/CommandPalette";
import { Menu } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";
import { ErrorBoundary } from "@/components/common/ErrorBoundary";

interface AppLayoutProps {
  children: ReactNode;
  workspace?: boolean;
}

export const AppLayout = forwardRef<HTMLDivElement, AppLayoutProps>(({ children, workspace = false }, ref) => {
  const [sidebarOpen, setSidebarOpen] = useState(false);

  return (
    <div
      ref={ref}
      className="min-h-[100dvh] app-canvas relative overflow-x-hidden touch-manipulation"
    >
      <div className="flex min-h-[100dvh] h-[100dvh]">
        <aside className="hidden lg:flex w-[236px] flex-col overflow-hidden border-r border-sidebar-border bg-sidebar flex-shrink-0">
          <AppSidebar />
        </aside>

        <div className="flex-1 flex flex-col min-w-0 min-h-0">
          <header className="h-14 min-h-14 border-b border-border/70 bg-card/80 backdrop-blur-xl flex items-center px-3 sm:px-5 gap-2 sm:gap-3 safe-area-top z-10">
            <Sheet open={sidebarOpen} onOpenChange={setSidebarOpen}>
              <SheetTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="lg:hidden h-9 w-9 min-h-9 min-w-9 touch-manipulation rounded-lg"
                  aria-label="打开菜单"
                >
                  <Menu className="h-5 w-5" />
                </Button>
              </SheetTrigger>
              <SheetContent
                side="left"
                className="w-[min(86vw,272px)] p-0 bg-sidebar border-r border-sidebar-border"
              >
                <AppSidebar onNavigate={() => setSidebarOpen(false)} />
              </SheetContent>
            </Sheet>

            <TopBar />
          </header>

          <main className={`flex-1 min-h-0 px-3.5 py-4 sm:px-5 sm:py-5 lg:px-7 lg:py-6 pb-[calc(4.75rem+env(safe-area-inset-bottom,0px))] lg:pb-7 overscroll-y-contain ${workspace ? "overflow-hidden" : "overflow-y-auto overflow-x-hidden"}`}>
            <div className={`max-w-[1440px] mx-auto w-full ${workspace ? "h-full min-h-0" : "matrix-fade-in"}`}>
              <ErrorBoundary>
                {children}
              </ErrorBoundary>
            </div>
          </main>

          <footer className="hidden lg:block h-8 shrink-0 border-t border-border/70 bg-card/40">
            <StatusBar />
          </footer>
        </div>
      </div>

      <MobileBottomNav />
      <CommandPalette />
    </div>
  );
});

AppLayout.displayName = "AppLayout";
