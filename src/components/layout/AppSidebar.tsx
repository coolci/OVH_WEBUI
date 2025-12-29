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
  Terminal,
  Cpu,
  User,
  Zap,
  UserCog,
  BarChart3
} from "lucide-react";
import { cn } from "@/lib/utils";

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
      { to: "/performance", icon: BarChart3, label: "性能监控" },
    ],
  },
  {
    title: "抢购",
    items: [
      { to: "/queue", icon: ListOrdered, label: "抢购队列" },
      { to: "/history", icon: History, label: "购买历史" },
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

  return (
    <div className="h-full flex flex-col">
      {/* Logo/Title */}
      <div className="h-12 flex items-center px-4 border-b border-sidebar-border">
        <div className="flex items-center gap-2">
          <Terminal className="h-5 w-5 text-primary glow-pulse" />
          <span className="font-bold text-primary tracking-wider">OVH_SNIPER</span>
          <span className="text-xs text-muted-foreground">v2.0</span>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 overflow-y-auto py-4 px-2">
        {navGroups.map((group) => (
          <div key={group.title} className="mb-4">
            <div className="px-3 mb-2 text-xs font-medium text-muted-foreground uppercase tracking-wider flex items-center gap-2">
              <span className="text-primary/50">//</span>
              {group.title}
            </div>
            <div className="space-y-1">
              {group.items.map((item) => {
                const isActive = location.pathname === item.to;
                return (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    onClick={onNavigate}
                    className={cn(
                      "flex items-center gap-3 px-3 py-2 rounded-sm text-sm transition-all duration-200",
                      "hover:bg-secondary hover:text-primary",
                      isActive 
                        ? "bg-primary/10 text-primary border-l-2 border-primary shadow-glow-sm" 
                        : "text-sidebar-foreground border-l-2 border-transparent"
                    )}
                  >
                    <item.icon className={cn(
                      "h-4 w-4 transition-all",
                      isActive && "text-primary drop-shadow-[0_0_8px_hsl(var(--primary))]"
                    )} />
                    <span>{item.label}</span>
                    {isActive && (
                      <span className="ml-auto text-primary cursor-blink">_</span>
                    )}
                  </NavLink>
                );
              })}
            </div>
          </div>
        ))}
      </nav>

      {/* Quick actions */}
      <div className="p-2 border-t border-sidebar-border">
        <button className="w-full flex items-center gap-2 px-3 py-2 rounded-sm text-sm text-accent hover:bg-accent/10 transition-colors">
          <Zap className="h-4 w-4" />
          <span>快速下单</span>
        </button>
      </div>
    </div>
  );
}
