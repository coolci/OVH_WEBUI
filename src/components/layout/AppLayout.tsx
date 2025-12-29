import { ReactNode, useState, forwardRef } from "react";
import { AppSidebar } from "./AppSidebar";
import { TopBar } from "./TopBar";
import { StatusBar } from "./StatusBar";
import { MobileBottomNav } from "./MobileBottomNav";
import { Menu } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";

interface AppLayoutProps {
  children: ReactNode;
}

export const AppLayout = forwardRef<HTMLDivElement, AppLayoutProps>(({ children }, ref) => {
  const [sidebarOpen, setSidebarOpen] = useState(false);

  return (
    <div ref={ref} className="min-h-screen bg-background grid-lines relative overflow-hidden">
      {/* Scanline overlay */}
      <div className="fixed inset-0 pointer-events-none z-50 scanlines opacity-50" />
      
      {/* Desktop layout */}
      <div className="flex h-screen">
        {/* Desktop Sidebar */}
        <aside className="hidden lg:flex w-64 flex-col border-r border-border bg-sidebar">
          <AppSidebar />
        </aside>

        {/* Main content area */}
        <div className="flex-1 flex flex-col min-w-0">
          {/* Top bar */}
          <header className="h-12 border-b border-border bg-card/50 backdrop-blur-sm flex items-center px-3 sm:px-4 gap-2 sm:gap-4">
            {/* Mobile menu button */}
            <Sheet open={sidebarOpen} onOpenChange={setSidebarOpen}>
              <SheetTrigger asChild>
                <Button variant="ghost" size="icon" className="lg:hidden h-8 w-8">
                  <Menu className="h-5 w-5" />
                </Button>
              </SheetTrigger>
              <SheetContent side="left" className="w-64 p-0 bg-sidebar border-r border-border">
                <AppSidebar onNavigate={() => setSidebarOpen(false)} />
              </SheetContent>
            </Sheet>
            
            <TopBar />
          </header>

          {/* Page content - add bottom padding for mobile nav */}
          <main className="flex-1 overflow-auto p-3 sm:p-4 md:p-6 pb-20 lg:pb-6">
            <div className="matrix-fade-in">
              {children}
            </div>
          </main>

          {/* Status bar - hidden on mobile */}
          <footer className="hidden lg:block h-8 border-t border-border bg-card/50 backdrop-blur-sm">
            <StatusBar />
          </footer>
        </div>
      </div>

      {/* Mobile Bottom Navigation */}
      <MobileBottomNav />
    </div>
  );
});

AppLayout.displayName = "AppLayout";
