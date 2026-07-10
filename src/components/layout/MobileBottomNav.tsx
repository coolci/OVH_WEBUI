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
  BarChart3,
  MessageSquare,
  UserCog,
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
  { to: "/vps-monitor", icon: MonitorDot, label: "VPS 监控", group: "监控" },
  { to: "/vps-control", icon: Cloud, label: "VPS 控制", group: "实例" },
  { to: "/performance", icon: BarChart3, label: "性能监控", group: "实例" },
  { to: "/history", icon: History, label: "购买历史", group: "抢购" },
  { to: "/telegram-order", icon: MessageSquare, label: "TG 下单", group: "抢购" },
  { to: "/account", icon: User, label: "账户管理", group: "配置" },
  { to: "/contact-change", icon: UserCog, label: "联系人变更", group: "配置" },
  { to: "/logs", icon: ScrollText, label: "系统日志", group: "配置" },
  { to: "/settings", icon: Settings, label: "系统设置", group: "配置" },
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
      <nav className="lg:hidden fixed bottom-0 left-0 right-0 z-40 bg-sidebar/90 backdrop-blur-xl border-t border-border/80 safe-area-bottom shadow-[0_-8px_32px_-12px_hsl(0_0%_0%/0.45)]">
        <div className="flex items-stretch justify-around min-h-[3.5rem]">
          {primaryNav.map((item) => {
            const isActive = location.pathname === item.to;
            return (
              <NavLink
                key={item.to}
                to={item.to}
                className={cn(
                  "relative flex flex-col items-center justify-center gap-0.5 px-1 py-2 min-w-0 flex-1 touch-manipulation select-none",
                  "min-h-[48px] active:bg-secondary/40 transition-colors",
                  isActive ? "text-primary" : "text-muted-foreground"
                )}
              >
                <item.icon
                  className={cn(
                    "h-5 w-5 transition-all",
                    isActive && "drop-shadow-[0_0_6px_hsl(var(--primary))]"
                  )}
                />
                <span className="text-[10px] font-medium truncate max-w-full px-0.5">
                  {item.label}
                </span>
                {isActive && (
                  <span className="absolute top-1 w-1 h-1 rounded-full bg-primary" />
                )}
              </NavLink>
            );
          })}

          <button
            type="button"
            onClick={() => setMoreOpen(true)}
            className={cn(
              "relative flex flex-col items-center justify-center gap-0.5 px-1 py-2 min-w-0 flex-1 touch-manipulation select-none",
              "min-h-[48px] active:bg-secondary/40 transition-colors",
              moreActive ? "text-primary" : "text-muted-foreground"
            )}
          >
            <MoreHorizontal className="h-5 w-5" />
            <span className="text-[10px] font-medium">更多</span>
            {moreActive && (
              <span className="absolute top-1 w-1 h-1 rounded-full bg-primary" />
            )}
          </button>
        </div>
      </nav>

      <Sheet open={moreOpen} onOpenChange={setMoreOpen}>
        <SheetContent
          side="bottom"
          className="max-h-[75dvh] rounded-t-2xl border-border bg-sidebar p-0 safe-area-bottom"
        >
          <SheetHeader className="px-4 pt-4 pb-2 border-b border-border text-left">
            <SheetTitle className="text-base">全部功能</SheetTitle>
          </SheetHeader>
          <div className="overflow-y-auto max-h-[calc(75dvh-4rem)] p-3 space-y-4">
            {Object.entries(groups).map(([group, items]) => (
              <div key={group}>
                <div className="text-[11px] uppercase tracking-wider text-muted-foreground px-2 mb-1.5">
                  {group}
                </div>
                <div className="grid grid-cols-3 sm:grid-cols-4 gap-2">
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
                          "flex flex-col items-center gap-1.5 p-3 rounded-xl border text-center touch-manipulation min-h-[72px]",
                          "active:scale-[0.98] transition-all",
                          active
                            ? "border-primary/50 bg-primary/10 text-primary"
                            : "border-border bg-card/40 text-foreground hover:bg-secondary/50"
                        )}
                      >
                        <item.icon className="h-5 w-5" />
                        <span className="text-[11px] font-medium leading-tight">
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
