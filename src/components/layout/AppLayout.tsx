import { ReactNode, useState } from "react";
import { AppSidebar } from "./AppSidebar";
import { TopBar } from "./TopBar";
import { StatusBar } from "./StatusBar";
import { Menu } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetTrigger } from "@/components/ui/sheet";

interface AppLayoutProps {
  children: ReactNode;
}

export function AppLayout({ children }: AppLayoutProps) {
  const [sidebarOpen, setSidebarOpen] = useState(false);

  return (
    <div className="min-h-screen bg-background grid-lines relative overflow-hidden">
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
          <header className="h-12 border-b border-border bg-card/50 backdrop-blur-sm flex items-center px-4 gap-4">
            {/* Mobile menu button */}
            <Sheet open={sidebarOpen} onOpenChange={setSidebarOpen}>
              <SheetTrigger asChild>
                <Button variant="ghost" size="icon" className="lg:hidden">
                  <Menu className="h-5 w-5" />
                </Button>
              </SheetTrigger>
              <SheetContent side="left" className="w-64 p-0 bg-sidebar border-r border-border">
                <AppSidebar onNavigate={() => setSidebarOpen(false)} />
              </SheetContent>
            </Sheet>
            
            <TopBar />
          </header>

          {/* Page content */}
          <main className="flex-1 overflow-auto p-4 md:p-6">
            <div className="matrix-fade-in">
              {children}
            </div>
          </main>

          {/* Status bar */}
          <footer className="h-8 border-t border-border bg-card/50 backdrop-blur-sm">
            <StatusBar />
          </footer>
        </div>
      </div>
    </div>
  );
}
