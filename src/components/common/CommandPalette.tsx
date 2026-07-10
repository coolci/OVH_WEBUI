import { useEffect, useState } from "react";
import { Command } from "cmdk";
import { useNavigate } from "react-router-dom";
import {
  BarChart3,
  Server,
  ClipboardList,
  Bell,
  Cloud,
  Terminal,
  User,
  Clock,
  FileText,
  Settings,
  Search,
  Cpu,
} from "lucide-react";

interface NavEntry {
  to: string;
  label: string;
  group: string;
  icon: React.ComponentType<{ className?: string }>;
  shortcut?: string;
}

const NAV_ENTRIES: NavEntry[] = [
  { to: "/", label: "仪表盘", group: "概览", icon: BarChart3, shortcut: "G D" },
  { to: "/servers", label: "服务器列表", group: "抢购", icon: Server, shortcut: "G S" },
  { to: "/queue", label: "抢购队列", group: "抢购", icon: ClipboardList, shortcut: "G Q" },
  { to: "/monitor", label: "服务器监控", group: "监控", icon: Bell, shortcut: "G M" },
  { to: "/vps-monitor", label: "VPS 补货", group: "监控", icon: Cloud, shortcut: "G V" },
  { to: "/server-control", label: "服务器控制", group: "实例", icon: Terminal, shortcut: "G C" },
  { to: "/vps-control", label: "VPS 控制", group: "实例", icon: Cpu },
  { to: "/account", label: "账户管理", group: "实例", icon: User },
  { to: "/history", label: "抢购历史", group: "系统", icon: Clock, shortcut: "G H" },
  { to: "/logs", label: "详细日志", group: "系统", icon: FileText, shortcut: "G L" },
  { to: "/settings", label: "API 设置", group: "系统", icon: Settings },
];

/**
 * ⌘K 命令面板（cmdk）：
 * - ⌘K / Ctrl+K 唤出
 * - 模糊搜索页面，回车跳转
 * - 全局快捷键 G + 字母 跳页面
 */
export function CommandPalette() {
  const [open, setOpen] = useState(false);
  const navigate = useNavigate();

  useEffect(() => {
    const isMac = navigator.platform.toLowerCase().includes("mac");
    const handler = (e: KeyboardEvent) => {
      if (e.key === "k" && (isMac ? e.metaKey : e.ctrlKey)) {
        e.preventDefault();
        setOpen((v) => !v);
      }
      if (e.key === "Escape") setOpen(false);
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);

  useEffect(() => {
    let waiting = false;
    let timer: ReturnType<typeof setTimeout> | null = null;
    const handler = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement)?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || (e.target as HTMLElement)?.isContentEditable) return;
      if (e.metaKey || e.ctrlKey || e.altKey) return;

      if (!waiting && e.key.toLowerCase() === "g") {
        waiting = true;
        if (timer) clearTimeout(timer);
        timer = setTimeout(() => {
          waiting = false;
        }, 1200);
        return;
      }
      if (waiting) {
        const map: Record<string, string> = {
          d: "/",
          s: "/servers",
          q: "/queue",
          m: "/monitor",
          v: "/vps-monitor",
          c: "/server-control",
          h: "/history",
          l: "/logs",
        };
        const target = map[e.key.toLowerCase()];
        if (target) {
          e.preventDefault();
          navigate(target);
        }
        waiting = false;
        if (timer) clearTimeout(timer);
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [navigate]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 bg-black/40 flex items-start justify-center pt-[12vh] px-4"
      onClick={() => setOpen(false)}
    >
      <Command
        className="w-full max-w-xl rounded-2xl bg-background border border-border overflow-hidden"
        onClick={(e) => e.stopPropagation()}
        loop
      >
        <div className="flex items-center gap-2 px-4 border-b border-border">
          <Search className="w-4 h-4 text-muted-foreground" />
          <Command.Input
            autoFocus
            placeholder="搜索页面或操作..."
            className="flex-1 h-12 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
          />
          <kbd className="px-1.5 py-0.5 rounded bg-muted text-[10px] font-mono text-muted-foreground">
            ESC
          </kbd>
        </div>
        <Command.List className="max-h-[60vh] overflow-y-auto p-2">
          <Command.Empty className="py-8 text-center text-sm text-muted-foreground">
            未找到匹配项
          </Command.Empty>
          {Object.entries(
            NAV_ENTRIES.reduce<Record<string, NavEntry[]>>((acc, e) => {
              (acc[e.group] = acc[e.group] || []).push(e);
              return acc;
            }, {})
          ).map(([group, entries]) => (
            <Command.Group
              key={group}
              heading={group}
              className="text-[11px] text-muted-foreground [&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:py-1.5"
            >
              {entries.map((entry) => {
                const Icon = entry.icon;
                return (
                  <Command.Item
                    key={entry.to}
                    value={`${entry.label} ${entry.to}`}
                    onSelect={() => {
                      navigate(entry.to);
                      setOpen(false);
                    }}
                    className="flex items-center gap-2 px-2 py-2 rounded-md text-sm text-foreground cursor-pointer data-[selected=true]:bg-muted"
                  >
                    <Icon className="w-4 h-4 text-muted-foreground" />
                    <span className="flex-1">{entry.label}</span>
                    {entry.shortcut && (
                      <span className="text-[10px] text-muted-foreground font-mono">
                        {entry.shortcut}
                      </span>
                    )}
                  </Command.Item>
                );
              })}
            </Command.Group>
          ))}
        </Command.List>
      </Command>
    </div>
  );
}
