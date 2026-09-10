import { ReactNode, useState, forwardRef } from "react";
import { AppSidebar } from "./AppSidebar";
import { TopBar } from "./TopBar";
import { StatusBar } from "./StatusBar";
import { MobileBottomNav } from "./MobileBottomNav";
import { CommandPalette } from "@/components/common/CommandPalette";
import { AuthorWatermark } from "@/components/common/AuthorWatermark";
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
      className="min-h-[100dvh] app-canvas relative overflow-x-hidden touch-manipulation"
    >
      <div className="flex min-h-[100dvh] h-[100dvh]">
        <aside className="hidden lg:flex w-[260px] flex-col overflow-hidden border-r border-sidebar-border bg-sidebar flex-shrink-0">
          <AppSidebar />
        </aside>

        <div className="flex-1 flex flex-col min-w-0 min-h-0">
          <header className="h-14 min-h-14 border-b border-border/70 bg-card/75 backdrop-blur-xl flex items-center px-2 sm:px-4 gap-1 sm:gap-3 safe-area-top z-10">
            <Sheet open={sidebarOpen} onOpenChange={setSidebarOpen}>
              <SheetTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="lg:hidden h-10 w-10 min-h-10 min-w-10 touch-manipulation rounded-lg"
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

          <main className="flex-1 overflow-y-auto overflow-x-hidden p-3.5 sm:p-5 md:p-6 lg:p-8 pb-[calc(4.5rem+env(safe-area-inset-bottom,0px))] lg:pb-8 overscroll-y-contain flex flex-col">
            <div className="matrix-fade-in max-w-[1520px] mx-auto w-full flex-1 flex flex-col justify-between">
              <div className="space-y-1">
                {children}
              </div>

              {/* 页面底部自然吸底的致敬印章 (真·沉底排版，头像原彩，绝不悬浮或折行) */}
              <footer className="mt-auto pt-10 pb-2 text-center select-none">
                <div className="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full border border-border/70 bg-card/60 backdrop-blur-md text-[11px] sm:text-[11.5px] font-mono shadow-sm max-w-full overflow-hidden">
                  <img
                    src="/author-avatar.png"
                    alt="cola"
                    className="w-5 h-5 rounded-full object-cover ring-1 ring-amber-500/70 shadow-sm flex-shrink-0"
                  />
                  
                  {/* 桌面端完整文案 */}
                  <span className="hidden sm:inline whitespace-nowrap text-muted-foreground font-medium">致敬开源作者:</span>
                  {/* 移动端精简文案 */}
                  <span className="sm:hidden whitespace-nowrap text-muted-foreground font-medium">鸣谢:</span>

                  <a
                    href="https://t.me/gocola"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="whitespace-nowrap text-foreground font-semibold hover:text-primary transition-colors flex items-center gap-1 flex-shrink-0"
                    title="Telegram: @gocola"
                  >
                    cola <span className="text-sky-400 font-mono text-[10.5px] font-normal">(@gocola)</span>
                  </a>

                  <span className="text-border/80 flex-shrink-0">·</span>

                  <a
                    href="https://github.com/gokele/ovh"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="whitespace-nowrap text-muted-foreground hover:text-foreground transition-colors underline decoration-border/80 hover:decoration-foreground underline-offset-2 flex-shrink-0"
                    title="GitHub: gokele/ovh"
                  >
                    <span className="hidden sm:inline">github.com/gokele/ovh</span>
                    <span className="sm:hidden">gokele/ovh</span>
                  </a>
                </div>
              </footer>
            </div>
          </main>

          <footer className="hidden lg:block h-9 border-t border-border/80 bg-card/40">
            <StatusBar />
          </footer>
        </div>
      </div>

      <AuthorWatermark />
      <MobileBottomNav />
      <CommandPalette />
    </div>
  );
});

AppLayout.displayName = "AppLayout";
