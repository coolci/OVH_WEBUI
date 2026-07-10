import { ReactNode, useState, forwardRef } from "react";
import { AppSidebar } from "./AppSidebar";
import { TopBar } from "./TopBar";
import { StatusBar } from "./StatusBar";
import { MobileBottomNav } from "./MobileBottomNav";
import { CommandPalette } from "@/components/common/CommandPalette";
import { Menu } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";

interface AppLayoutProps {
  children: ReactNode;
}

export const AppLayout = forwardRef<HTMLDivElement, AppLayoutProps>(({ children }, ref) => {
  const [sidebarOpen, setSidebarOpen] = useState(false);

  return (
    <div
      ref={ref}
      className="min-h-[100dvh] app-canvas grid-lines relative overflow-x-hidden touch-manipulation"
    >
      <div className="fixed inset-0 pointer-events-none z-[1] scanlines opacity-30" aria-hidden />

      <div className="flex min-h-[100dvh] h-[100dvh]">
        <aside className="hidden lg:flex w-[260px] flex-col border-r border-sidebar-border/90 bg-sidebar/95 backdrop-blur-xl flex-shrink-0 shadow-[4px_0_24px_-12px_hsl(0_0%_0%/0.4)]">
          <AppSidebar />
        </aside>

        <div className="flex-1 flex flex-col min-w-0 min-h-0">
          <header className="h-14 min-h-14 border-b border-border/80 bg-card/40 backdrop-blur-xl flex items-center px-2 sm:px-4 gap-1 sm:gap-3 safe-area-top z-10">
            <Sheet open={sidebarOpen} onOpenChange={setSidebarOpen}>
              <SheetTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="lg:hidden h-10 w-10 min-h-10 min-w-10 touch-manipulation rounded-xl"
                  aria-label="打开菜单"
                >
                  <Menu className="h-5 w-5" />
                </Button>
              </SheetTrigger>
              <SheetContent
                side="left"
                className="w-[min(88vw,300px)] p-0 bg-sidebar border-r border-sidebar-border"
              >
                <AppSidebar onNavigate={() => setSidebarOpen(false)} />
              </SheetContent>
            </Sheet>

            <TopBar />
          </header>

          <main className="flex-1 overflow-y-auto overflow-x-hidden p-3 sm:p-5 md:p-6 lg:p-8 pb-[calc(4.75rem+env(safe-area-inset-bottom,0px))] lg:pb-8 overscroll-y-contain">
            <div className="matrix-fade-in max-w-[1520px] mx-auto w-full space-y-1">
              {children}
            </div>
          </main>

          <footer className="hidden lg:block h-9 border-t border-border/80 bg-card/30 backdrop-blur-md">
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
