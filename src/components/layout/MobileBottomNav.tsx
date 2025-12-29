import { NavLink, useLocation } from "react-router-dom";
import { 
  LayoutDashboard, 
  Server, 
  ListOrdered, 
  Activity,
  Settings
} from "lucide-react";
import { cn } from "@/lib/utils";

const navItems = [
  { to: "/", icon: LayoutDashboard, label: "仪表盘" },
  { to: "/servers", icon: Server, label: "服务器" },
  { to: "/queue", icon: ListOrdered, label: "队列" },
  { to: "/monitor", icon: Activity, label: "监控" },
  { to: "/settings", icon: Settings, label: "设置" },
];

export function MobileBottomNav() {
  const location = useLocation();

  return (
    <nav className="lg:hidden fixed bottom-0 left-0 right-0 z-40 bg-sidebar/95 backdrop-blur-md border-t border-border safe-area-bottom">
      <div className="flex items-center justify-around h-14">
        {navItems.map((item) => {
          const isActive = location.pathname === item.to;
          return (
            <NavLink
              key={item.to}
              to={item.to}
              className={cn(
                "flex flex-col items-center justify-center gap-0.5 px-3 py-1.5 min-w-0 flex-1 transition-colors",
                isActive 
                  ? "text-primary" 
                  : "text-muted-foreground hover:text-foreground"
              )}
            >
              <item.icon className={cn(
                "h-5 w-5 transition-all",
                isActive && "drop-shadow-[0_0_6px_hsl(var(--primary))]"
              )} />
              <span className={cn(
                "text-[10px] font-medium truncate",
                isActive && "text-primary"
              )}>
                {item.label}
              </span>
              {isActive && (
                <span className="absolute bottom-1 w-1 h-1 rounded-full bg-primary" />
              )}
            </NavLink>
          );
        })}
      </div>
    </nav>
  );
}
