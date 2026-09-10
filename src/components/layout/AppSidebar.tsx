import { useState } from "react";
import { NavLink, useLocation } from "react-router-dom";
import {
  LayoutDashboard,
  Server,
  ListOrdered,
  History,
  Activity,
  MonitorDot,
  Settings,
  ScrollText,
  Cpu,
  User,
  Cloud,
  CloudLightning,
  Github,
  Flame,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { QuickOrderDialog } from "@/components/orders/QuickOrderDialog";
import { BrandBar } from "./BrandBar";

interface AppSidebarProps {
  onNavigate?: () => void;
}

const navGroups = [
  {
    title: "概览",
    items: [
      { to: "/", icon: LayoutDashboard, label: "仪表盘" },
    ],
  },
  {
    title: "抢购",
    items: [
      { to: "/servers", icon: Server, label: "服务器列表" },
      { to: "/queue", icon: ListOrdered, label: "抢购队列" },
      { to: "/telegram-order", icon: CloudLightning, label: "云下单" },
    ],
  },
  {
    title: "监控",
    items: [
      { to: "/monitor", icon: Activity, label: "服务器监控" },
      { to: "/vps-monitor", icon: MonitorDot, label: "VPS 补货" },
    ],
  },
  {
    title: "实例",
    items: [
      { to: "/server-control", icon: Cpu, label: "服务器控制" },
      { to: "/vps-control", icon: Cloud, label: "VPS 控制" },
      { to: "/account", icon: User, label: "账户管理" },
    ],
  },
  {
    title: "系统",
    items: [
      { to: "/history", icon: History, label: "抢购历史" },
      { to: "/logs", icon: ScrollText, label: "系统日志" },
      { to: "/settings", icon: Settings, label: "系统设置" },
    ],
  },
];

export function AppSidebar({ onNavigate }: AppSidebarProps) {
  const location = useLocation();
  const [quickOrderOpen, setQuickOrderOpen] = useState(false);

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden bg-sidebar">
      <BrandBar onQuickOrder={() => setQuickOrderOpen(true)} />

      {/* Nav */}
      <nav className="flex-1 overflow-y-auto py-3 px-2.5">
        {navGroups.map((group) => (
          <div key={group.title} className="mb-4">
            <div className="section-label px-2.5 mb-1.5 flex items-center gap-2">
              <span className="h-px flex-1 bg-border/60 max-w-[12px]" />
              {group.title}
            </div>
            <div className="space-y-0.5">
              {group.items.map((item) => {
                const isActive = location.pathname === item.to;
                return (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    onClick={onNavigate}
                    className={cn(
                      "group relative flex items-center gap-2.5 px-3 py-2 rounded-xl text-[13px] font-medium transition-all duration-150",
                      isActive
                        ? "bg-primary/10 text-primary font-semibold"
                        : "text-sidebar-foreground/75 hover:bg-sidebar-accent/60 hover:text-foreground"
                    )}
                  >
                    <item.icon
                      className={cn(
                        "h-[1.05rem] w-[1.05rem] transition-colors flex-shrink-0",
                        isActive
                          ? "text-primary"
                          : "text-muted-foreground/80 group-hover:text-foreground"
                      )}
                      strokeWidth={isActive ? 2.2 : 1.75}
                    />
                    <span className="truncate">{item.label}</span>
                    {item.to === "/telegram-order" && !isActive && (
                      <span className="ml-auto text-[10px] font-mono px-1.5 py-0.5 rounded bg-sky-500/10 text-sky-400/90 font-medium leading-none">
                        TG
                      </span>
                    )}
                    {isActive && (
                      <span className="ml-auto h-1.5 w-1.5 rounded-full bg-primary" />
                    )}
                  </NavLink>
                );
              })}
            </div>
          </div>
        ))}
      </nav>

      <div className="shrink-0 border-t border-sidebar-border p-3 space-y-2">
        {/* 致敬开源作者 */}
        <div className="rounded-xl border border-white/10 bg-gradient-to-br from-card/90 to-muted/40 p-2.5 shadow-sm relative overflow-hidden group">
          <div className="flex items-center gap-2.5">
            <img
              src="/author-avatar.png"
              alt="cola"
              className="w-8 h-8 rounded-full object-cover ring-1 ring-amber-500/40 flex-shrink-0"
            />
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-1 text-[10px] text-muted-foreground font-medium uppercase tracking-wider">
                <Flame className="w-2.5 h-2.5 text-amber-500 fill-amber-500/30" />
                <span>致敬开源作者</span>
              </div>
              <div className="flex items-center justify-between gap-1 mt-0.5">
                <a
                  href="https://t.me/gocola"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-xs font-semibold text-foreground hover:text-primary transition-colors truncate"
                  title="Telegram: @gocola"
                >
                  cola <span className="text-[10px] text-sky-400 font-normal font-mono">(@gocola)</span>
                </a>
                <a
                  href="https://github.com/gokele/ovh"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-muted-foreground hover:text-foreground transition-colors p-1 rounded hover:bg-muted"
                  title="GitHub: gokele/ovh"
                >
                  <Github className="w-3.5 h-3.5" />
                </a>
              </div>
            </div>
          </div>
        </div>

        <div className="flex items-center justify-between rounded-lg border border-border/70 bg-muted/30 px-3 py-2 text-[11px] leading-relaxed text-muted-foreground">
          <div className="flex items-center gap-1.5">
            <span className="h-1.5 w-1.5 rounded-full bg-primary" />
            <span className="font-medium text-foreground/85">集群自托管</span>
          </div>
          <span className="font-mono text-[10px] text-muted-foreground/70">多账户</span>
        </div>
      </div>

      <QuickOrderDialog open={quickOrderOpen} onOpenChange={setQuickOrderOpen} />
    </div>
  );
}
