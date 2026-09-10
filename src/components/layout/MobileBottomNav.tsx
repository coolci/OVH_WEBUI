import { NavLink, useLocation, useNavigate } from "react-router-dom";
import {
  LayoutDashboard,
  Server,
  ListOrdered,
  Cpu,
  MoreHorizontal,
  Activity,
  MonitorDot,
  Cloud,
  History,
  ScrollText,
  Settings,
  User,
  CloudLightning,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useState } from "react";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";

const primaryNav = [
  { to: "/", icon: LayoutDashboard, label: "首页" },
  { to: "/servers", icon: Server, label: "列表" },
  { to: "/server-control", icon: Cpu, label: "控制" },
  { to: "/queue", icon: ListOrdered, label: "队列" },
];

const moreNav = [
  { to: "/monitor", icon: Activity, label: "独服监控", group: "监控" },
  { to: "/vps-monitor", icon: MonitorDot, label: "VPS 补货", group: "监控" },
  { to: "/vps-control", icon: Cloud, label: "VPS 控制", group: "实例" },
  { to: "/account", icon: User, label: "账户管理", group: "实例" },
  { to: "/telegram-order", icon: CloudLightning, label: "云下单", group: "抢购" },
  { to: "/history", icon: History, label: "抢购历史", group: "系统" },
  { to: "/logs", icon: ScrollText, label: "详细日志", group: "系统" },
  { to: "/settings", icon: Settings, label: "系统设置", group: "系统" },
];

export function MobileBottomNav() {
  const location = useLocation();
  const navigate = useNavigate();
  const [moreOpen, setMoreOpen] = useState(false);

  const moreActive = moreNav.some((i) => location.pathname === i.to);

  const groups = moreNav.reduce<Record<string, typeof moreNav>>((acc, item) => {
    (acc[item.group] = acc[item.group] || []).push(item);
    return acc;
  }, {});

  return (
    <>
      <nav className="lg:hidden fixed bottom-0 left-0 right-0 z-40 bg-card/85 backdrop-blur-2xl border-t border-border/70 safe-area-bottom shadow-[0_-4px_24px_rgba(0,0,0,0.5)]">
        <div className="flex items-stretch justify-around min-h-[3.6rem] px-1 py-1">
          {primaryNav.map((item) => {
            const isActive = location.pathname === item.to;
            return (
              <NavLink
                key={item.to}
                to={item.to}
                className={cn(
                  "relative flex flex-col items-center justify-center gap-1 px-2 py-1 min-w-0 flex-1 touch-manipulation select-none rounded-xl transition-all duration-200",
                  "active:scale-95",
                  isActive
                    ? "text-primary font-medium"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                <div className={cn(
                  "relative flex items-center justify-center w-8 h-7 rounded-lg transition-all",
                  isActive && "bg-primary/10"
                )}>
                  <item.icon className={cn("h-4.5 w-4.5 transition-transform duration-200", isActive && "scale-105")} />
                </div>
                <span className="text-[10.5px] tracking-tight truncate max-w-full leading-none">
                  {item.label}
                </span>
                {isActive && (
                  <span className="absolute -bottom-0.5 w-4 h-0.5 rounded-full bg-primary" />
                )}
              </NavLink>
            );
          })}

          <button
            type="button"
            onClick={() => setMoreOpen(true)}
            className={cn(
              "relative flex flex-col items-center justify-center gap-1 px-2 py-1 min-w-0 flex-1 touch-manipulation select-none rounded-xl transition-all duration-200",
              "active:scale-95",
              moreActive
                ? "text-primary font-medium"
                : "text-muted-foreground hover:text-foreground"
            )}
          >
            <div className={cn(
              "relative flex items-center justify-center w-8 h-7 rounded-lg transition-all",
              moreActive && "bg-primary/10"
            )}>
              <MoreHorizontal className={cn("h-4.5 w-4.5 transition-transform duration-200", moreActive && "scale-105")} />
            </div>
            <span className="text-[10.5px] tracking-tight leading-none">更多</span>
            {moreActive && (
              <span className="absolute -bottom-0.5 w-4 h-0.5 rounded-full bg-primary" />
            )}
          </button>
        </div>
      </nav>

      <Sheet open={moreOpen} onOpenChange={setMoreOpen}>
        <SheetContent
          side="bottom"
          className="max-h-[80dvh] rounded-t-2xl border-t border-border bg-card p-0 safe-area-bottom shadow-2xl"
        >
          {/* 拖动横条 */}
          <div className="mx-auto mt-3 h-1 w-10 rounded-full bg-muted-foreground/30" />
          <SheetHeader className="px-5 pt-3 pb-3 border-b border-border text-left">
            <SheetTitle className="text-sm font-semibold tracking-tight">全部功能</SheetTitle>
          </SheetHeader>
          <div className="overflow-y-auto max-h-[calc(80dvh-5rem)] p-4 space-y-4">
            {Object.entries(groups).map(([group, items]) => (
              <div key={group} className="space-y-2">
                <div className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/75 px-1">
                  {group}
                </div>
                <div className="grid grid-cols-3 sm:grid-cols-4 gap-2.5">
                  {items.map((item) => {
                    const active = location.pathname === item.to;
                    return (
                      <button
                        key={item.to}
                        type="button"
                        onClick={() => {
                          navigate(item.to);
                          setMoreOpen(false);
                        }}
                        className={cn(
                          "flex flex-col items-center justify-center gap-2 p-3 rounded-xl border text-center touch-manipulation min-h-[72px]",
                          "active:scale-[0.96] transition-colors",
                          active
                            ? "border-primary bg-primary/10 text-primary font-medium"
                            : "border-border bg-secondary/30 text-foreground hover:bg-secondary/60 hover:border-border"
                        )}
                      >
                        <div className={cn(
                          "w-8 h-8 rounded-lg flex items-center justify-center transition-colors",
                          active ? "bg-primary/20 text-primary" : "bg-muted text-muted-foreground"
                        )}>
                          <item.icon className="h-4.5 w-4.5" />
                        </div>
                        <span className="text-[11.5px] font-medium leading-tight">
                          {item.label}
                        </span>
                      </button>
                    );
                  })}
                </div>
              </div>
            ))}
          </div>
        </SheetContent>
      </Sheet>
    </>
  );
}
