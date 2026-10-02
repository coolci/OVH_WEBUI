import React, { useRef, useState, useEffect } from "react";
import { Users, Server, Cloud, Check, ChevronLeft, ChevronRight, RefreshCw, Pencil } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { StatusDot } from "@/components/common/StatusDot";
import { maskSensitive } from "@/hooks/use-hide-ip";
import { cn } from "@/lib/utils";

export interface DeviceSwitcherAccount {
  id: string;
  name: string;
  zone?: string;
  isDefault?: boolean;
}

export interface DeviceSwitcherItem {
  serviceName: string;
  name: string;
  rawName?: string;
  datacenter?: string;
  subtext?: string;
  state: "ok" | "running" | "stopped" | "warning" | "error" | string;
}

export interface DeviceSwitcherCardProps {
  deviceType: "server" | "vps";
  accounts: DeviceSwitcherAccount[];
  activeAccount?: string;
  onAccountChange: (accountId: string) => void;

  items: DeviceSwitcherItem[];
  selectedName?: string;
  onSelectName: (serviceName: string) => void;

  isFetching?: boolean;
  hidden?: boolean;
  onRename?: (serviceName: string) => void;
  className?: string;
}

/**
 * 一体化设备导航控制台 (DeviceSwitcherCard)
 *
 * 解决原先"顶部下拉选择卡片"与"集群快速切换卡片"双层冗余、
 * 以及 2 台机器时横向溢出截断 (`ns529169.ip-1...`) 的严重排版缺陷。
 *
 * 规则：
 * - 2 台设备：严格 2 等分网格 (50% / 50%)，绝无横向截断，完全对称。
 * - 3 台设备：严格 3 等分网格 (移动端单列，平板/桌面端 3 列对称)。
 * - 4 台设备：移动端 2x2 对称，平板/桌面端 4 列对称。
 * - 5+ 台设备：提供顶栏快速下拉跳转 + 底栏支持左右平滑滚动和轮播。
 * - 1 台设备：仅展示状态与账户，不呈现冗余单项切换按钮。
 */
export function DeviceSwitcherCard({
  deviceType,
  accounts,
  activeAccount,
  onAccountChange,
  items,
  selectedName,
  onSelectName,
  isFetching = false,
  hidden = false,
  onRename,
  className,
}: DeviceSwitcherCardProps) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [canScrollLeft, setCanScrollLeft] = useState(false);
  const [canScrollRight, setCanScrollRight] = useState(false);

  const isServer = deviceType === "server";
  const deviceLabel = isServer ? "服务器" : "VPS";
  const DeviceIcon = isServer ? Server : Cloud;

  const count = items.length;

  const checkScroll = () => {
    const el = scrollRef.current;
    if (!el) return;
    setCanScrollLeft(el.scrollLeft > 4);
    setCanScrollRight(el.scrollLeft < el.scrollWidth - el.clientWidth - 4);
  };

  useEffect(() => {
    checkScroll();
  }, [items]);

  const handleScroll = (dir: "left" | "right") => {
    const el = scrollRef.current;
    if (!el) return;
    const offset = dir === "left" ? -240 : 240;
    el.scrollBy({ left: offset, behavior: "smooth" });
    setTimeout(checkScroll, 250);
  };

  const renderCard = (item: DeviceSwitcherItem) => {
    const isCurrent = item.serviceName === selectedName;
    const isOk = item.state === "ok" || item.state === "running";
    const isWarn = item.state === "stopped" || item.state === "warning";

    return (
      <div
        key={item.serviceName}
        className={cn(
          "group relative flex flex-col justify-between p-2.5 sm:p-3 rounded-xl border text-left transition-all duration-150 touch-manipulation min-w-0 select-none",
          isCurrent
            ? "border-primary/60 bg-primary/10 text-foreground shadow-sm ring-1 ring-primary/25"
            : "border-border/60 hover:border-border bg-card/60 hover:bg-muted/60 text-muted-foreground hover:text-foreground"
        )}
      >
        <button
          type="button"
          onClick={() => onSelectName(item.serviceName)}
          className="w-full text-left focus:outline-none"
          title={`点击切换至此${deviceLabel}`}
        >
          {/* 第一行: 状态点 + 主机名/别名 + 机房徽标 */}
          <div className="flex items-center justify-between gap-1.5 w-full min-w-0">
            <div className="flex items-center gap-2 min-w-0 flex-1">
              <StatusDot tone={isOk ? "success" : isWarn ? "warning" : "muted"} size="xs" />
              <span
                className={cn(
                  "font-mono text-xs truncate leading-tight tracking-tight",
                  isCurrent ? "font-bold text-foreground" : "font-medium text-foreground/80 group-hover:text-foreground"
                )}
              >
                {maskSensitive(item.name, hidden)}
              </span>
            </div>
            <span className="px-1.5 py-0.5 rounded text-[10px] font-mono uppercase font-semibold leading-none bg-muted/80 text-muted-foreground flex-shrink-0 border border-border/40">
              {(item.datacenter || "").toUpperCase() || "DC"}
            </span>
          </div>

          {/* 第二行: 型号/规格 + 状态标签 */}
          <div className="flex items-center justify-between text-[11px] text-muted-foreground mt-2 min-w-0 w-full">
            <span className="truncate pr-1">
              {item.subtext || (isServer ? "独立服务器" : "OVH VPS")}
            </span>
            {isCurrent ? (
              <span className="text-[10px] font-semibold text-primary flex items-center gap-0.5 flex-shrink-0">
                <Check className="w-3 h-3 text-primary stroke-[2.5]" />
                <span>当前</span>
              </span>
            ) : (
              <span className="text-[10px] text-muted-foreground/60 opacity-0 group-hover:opacity-100 transition-opacity flex-shrink-0">
                切换 ↗
              </span>
            )}
          </div>
        </button>

        {/* 快捷别名修改入口 (右键或悬浮点击) */}
        {onRename && (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onRename(item.serviceName);
            }}
            className="absolute top-2 right-12 opacity-0 group-hover:opacity-80 hover:!opacity-100 p-1 rounded hover:bg-muted text-muted-foreground transition-opacity"
            title="修改别名"
          >
            <Pencil className="w-3 h-3" />
          </button>
        )}
      </div>
    );
  };

  return (
    <Card className={cn("surface-card rounded-2xl border-border shadow-sm overflow-hidden", className)}>
      <CardContent className="p-3 sm:p-4 space-y-3">
        {/* 顶栏: 账户选择器 + 状态统计 + (多设备时的快速下拉跳转) */}
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5">
          <div className="flex items-center gap-2 flex-wrap min-w-0">
            {/* 账户切换 */}
            <div className="flex items-center gap-1.5 min-w-0">
              <span className="text-xs font-medium text-muted-foreground whitespace-nowrap flex items-center gap-1 flex-shrink-0">
                <Users className="w-3.5 h-3.5" />
                <span>账户</span>
              </span>
              <Select value={activeAccount || ""} onValueChange={onAccountChange}>
                <SelectTrigger className="h-8 rounded-lg text-xs border-border/80 bg-background/60 touch-manipulation min-w-[130px] sm:min-w-[170px]">
                  <SelectValue placeholder="选择账户" />
                </SelectTrigger>
                <SelectContent>
                  {accounts.map((a) => (
                    <SelectItem key={a.id} value={a.id}>
                      {a.name} {a.zone ? `· ${a.zone}` : ""}
                      {a.isDefault && <span className="ml-1.5 text-[10px] text-muted-foreground">(默认)</span>}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {/* 当设备数量 >= 5 时，在顶栏提供快速检索下拉，避免用户大范围横向翻页 */}
            {count >= 5 && (
              <div className="flex items-center gap-1.5 min-w-0">
                <span className="text-xs font-medium text-muted-foreground whitespace-nowrap flex items-center gap-1 flex-shrink-0">
                  <DeviceIcon className="w-3.5 h-3.5" />
                  <span>跳转</span>
                </span>
                <Select value={selectedName || ""} onValueChange={onSelectName}>
                  <SelectTrigger className="h-8 rounded-lg text-xs font-mono border-border/80 bg-background/60 touch-manipulation min-w-[160px] sm:min-w-[220px]">
                    <SelectValue placeholder={`快速查找 ${deviceLabel}`} />
                  </SelectTrigger>
                  <SelectContent className="max-h-[360px]">
                    {items.map((it) => (
                      <SelectItem key={it.serviceName} value={it.serviceName} className="font-mono text-xs">
                        <div className="flex items-center gap-2">
                          <StatusDot
                            tone={
                              it.state === "ok" || it.state === "running"
                                ? "success"
                                : it.state === "stopped" || it.state === "warning"
                                ? "warning"
                                : "muted"
                            }
                            size="xs"
                          />
                          <span className="font-semibold">{maskSensitive(it.name, hidden)}</span>
                          <span className="text-[10px] text-muted-foreground font-sans ml-1">
                            {(it.datacenter || "").toUpperCase()}
                          </span>
                        </div>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}
          </div>

          {/* 右侧统计信息 */}
          <div className="flex items-center justify-between sm:justify-end gap-3 text-xs text-muted-foreground pt-1.5 sm:pt-0 border-t sm:border-t-0 border-border/40 flex-shrink-0">
            <div className="flex items-center gap-2">
              <DeviceIcon className="w-3.5 h-3.5 text-muted-foreground/70" />
              <span>
                共 <strong className="text-foreground font-mono">{count}</strong> 台 {deviceLabel}
              </span>
            </div>
            {isFetching ? (
              <span className="text-primary text-[11px] font-medium flex items-center gap-1 animate-pulse">
                <RefreshCw className="w-3 h-3 animate-spin" />
                同步中…
              </span>
            ) : count > 0 ? (
              <span className="inline-flex items-center gap-1 text-[11px] text-success">
                <span className="w-1.5 h-1.5 rounded-full bg-success"></span>
                已连接
              </span>
            ) : null}
          </div>
        </div>

        {/* 底栏: 设备快速切换网格 (当有 2 台及以上机器时展示) */}
        {count > 1 && (
          <div className="pt-2.5 border-t border-border/50">
            {/* 2 台机器：严格 2 等分网格，彻底消灭横向截断 */}
            {count === 2 && (
              <div className="grid grid-cols-2 gap-2 sm:gap-2.5 w-full">
                {items.map(renderCard)}
              </div>
            )}

            {/* 3 台机器：移动端单列/三列，桌面端 3 列均衡网格 */}
            {count === 3 && (
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-2 w-full">
                {items.map(renderCard)}
              </div>
            )}

            {/* 4 台机器：移动端 2x2 对称网格，桌面端 4 列均衡网格 */}
            {count === 4 && (
              <div className="grid grid-cols-2 md:grid-cols-4 gap-2 w-full">
                {items.map(renderCard)}
              </div>
            )}

            {/* 5+ 台机器：带导航的平滑横向滚动条 */}
            {count >= 5 && (
              <div className="relative group/scroll">
                {canScrollLeft && (
                  <button
                    type="button"
                    onClick={() => handleScroll("left")}
                    className="absolute left-0 top-1/2 -translate-y-1/2 z-10 w-7 h-7 rounded-full bg-background/90 border border-border shadow-md flex items-center justify-center text-foreground hover:bg-muted transition-all"
                    title="向左滚动"
                  >
                    <ChevronLeft className="w-4 h-4" />
                  </button>
                )}

                <div
                  ref={scrollRef}
                  onScroll={checkScroll}
                  className="flex items-center gap-2 overflow-x-auto no-scrollbar scroll-smooth snap-x snap-mandatory pb-0.5"
                >
                  {items.map((it) => (
                    <div key={it.serviceName} className="min-w-[200px] sm:min-w-[240px] flex-1 snap-start">
                      {renderCard(it)}
                    </div>
                  ))}
                </div>

                {canScrollRight && (
                  <button
                    type="button"
                    onClick={() => handleScroll("right")}
                    className="absolute right-0 top-1/2 -translate-y-1/2 z-10 w-7 h-7 rounded-full bg-background/90 border border-border shadow-md flex items-center justify-center text-foreground hover:bg-muted transition-all"
                    title="向右滚动"
                  >
                    <ChevronRight className="w-4 h-4" />
                  </button>
                )}
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
