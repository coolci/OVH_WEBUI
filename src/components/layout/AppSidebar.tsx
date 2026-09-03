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
  UserCog,
  BarChart3,
  MessageSquare,
  Cloud,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { QuickOrderDialog } from "@/components/orders/QuickOrderDialog";
import { BrandBar } from "./BrandBar";

interface AppSidebarProps {
  onNavigate?: () => void;
}

const navGroups = [
  {
    title: "系统",
    items: [
      { to: "/", icon: LayoutDashboard, label: "仪表盘" },
      { to: "/logs", icon: ScrollText, label: "系统日志" },
    ],
  },
  {
    title: "服务器",
    items: [
      { to: "/servers", icon: Server, label: "服务器列表" },
      { to: "/server-control", icon: Cpu, label: "服务器控制" },
      { to: "/vps-control", icon: Cloud, label: "VPS 控制" },
      { to: "/performance", icon: BarChart3, label: "性能监控" },
    ],
  },
  {
    title: "抢购",
    items: [
      { to: "/queue", icon: ListOrdered, label: "抢购队列" },
      { to: "/history", icon: History, label: "购买历史" },
      { to: "/telegram-order", icon: MessageSquare, label: "Telegram 下单" },
    ],
  },
  {
    title: "监控",
    items: [
      { to: "/monitor", icon: Activity, label: "独服监控" },
      { to: "/vps-monitor", icon: MonitorDot, label: "VPS 监控" },
    ],
  },
  {
    title: "配置",
    items: [
      { to: "/account", icon: User, label: "账户管理" },
      { to: "/contact-change", icon: UserCog, label: "联系人变更" },
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
                      "group flex items-center gap-2.5 px-2.5 py-2 rounded-lg text-[13px] font-medium transition-all duration-200",
                      isActive
                        ? "bg-primary/12 text-primary shadow-[inset_0_0_0_1px_hsl(var(--primary)/0.18)]"
                        : "text-sidebar-foreground/85 hover:bg-muted/70 hover:text-foreground"
                    )}
                  >
                    <item.icon
                      className={cn(
                        "h-[1.05rem] w-[1.05rem] transition-colors flex-shrink-0",
                        isActive
                          ? "text-primary"
                          : "text-muted-foreground group-hover:text-foreground"
                      )}
                      strokeWidth={isActive ? 2.25 : 1.75}
                    />
                    <span className="truncate">{item.label}</span>
                    {isActive && (
                      <span className="ml-auto h-1.5 w-1.5 rounded-full bg-primary shadow-[0_0_8px_hsl(var(--primary))]" />
                    )}
                  </NavLink>
                );
              })}
            </div>
          </div>
        ))}
      </nav>

      <div className="shrink-0 border-t border-sidebar-border/80 p-3">
        <div className="rounded-lg border border-border/50 bg-muted/30 px-3 py-2 font-mono text-[10px] leading-relaxed text-muted-foreground">
          <span className="text-primary">●</span> 自托管
          <span className="mx-1.5 text-border">·</span>
          多账户
        </div>
      </div>

      <QuickOrderDialog open={quickOrderOpen} onOpenChange={setQuickOrderOpen} />
    </div>
  );
}
