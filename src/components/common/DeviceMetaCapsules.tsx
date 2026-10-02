import React from "react";
import {
  CalendarClock,
  CalendarPlus,
  Repeat,
  AlertTriangle,
  Undo2,
  ExternalLink,
} from "lucide-react";
import { OsIcon } from "@/components/server-control/OsIcon";
import { formatDate, formatOsDisplayName } from "@/lib/format-os";
import { cn } from "@/lib/utils";

export interface DeviceMetaCapsulesProps {
  /** 操作系统信息 */
  os?: {
    rawName?: string | null;
    distribution?: string;
    family?: string;
  } | null;
  /** 点击 OS 胶囊回调 (如打开系统重装 / 重建对话框) */
  onOsClick?: () => void;
  osClickTitle?: string;

  /** 到期时间 */
  expiration?: string | number | Date | null;

  /** 开通时间 */
  creation?: string | number | Date | null;

  /** 续费策略 */
  renewal?: {
    renewalDeleteAtExpiration?: boolean;
    renewalForced?: boolean;
    renewalType?: boolean | string;
    renewalPeriod?: number;
    formatted?: string;
  } | null;
  /** 点击续费胶囊回调 (如打开续费管理对话框) */
  onRenewalClick?: () => void;

  /** 可撤单信息 (针对新开独立服务器) */
  retraction?: {
    eligible: boolean;
    hoursLeft?: number;
    daysLeft?: number;
  } | null;
  /** 点击撤单回调 */
  onRetractClick?: () => void;

  className?: string;
}

/**
 * 设备元数据统一胶囊条 (DeviceMetaCapsules)
 *
 * 采用严格对称的响应式网格 (移动/平板 2x2 对称双列，桌面端 4 列均衡对称)，
 * 彻底消灭因浮动换行导致的多行不对称与孤行空缺问题。
 */
export function DeviceMetaCapsules({
  os,
  onOsClick,
  osClickTitle = "点击进入系统重装 / 重建",
  expiration,
  creation,
  renewal,
  onRenewalClick,
  retraction,
  onRetractClick,
  className,
}: DeviceMetaCapsulesProps) {
  // 解析续费文案与警示状态
  const isDeleteAtExpiration = Boolean(renewal?.renewalDeleteAtExpiration);
  let renewalText = "—";
  if (renewal) {
    if (isDeleteAtExpiration) {
      renewalText = "到期注销";
    } else if (renewal.renewalForced) {
      renewalText = "强制自动";
    } else if (renewal.renewalType === true || renewal.renewalType === "automatic") {
      renewalText = "自动";
    } else if (renewal.renewalType === false || renewal.renewalType === "manual") {
      renewalText = "手动";
    } else if (renewal.formatted) {
      renewalText = renewal.formatted;
    } else if (renewal.renewalType) {
      renewalText = String(renewal.renewalType);
    }
    if (!isDeleteAtExpiration && typeof renewal.renewalPeriod === "number" && renewal.renewalPeriod > 0) {
      renewalText += ` · ${renewal.renewalPeriod}月`;
    }
  }

  const rawOs = os?.rawName || "";
  const displayOs = formatOsDisplayName(rawOs);

  return (
    <div className={cn("pt-2.5 border-t border-border/50 space-y-2", className)}>
      {/* 1. 可撤单警示条 (仅当支持撤单时置顶全宽呈现，保持下方网格绝对对称) */}
      {retraction?.eligible && (
        <button
          type="button"
          onClick={onRetractClick}
          className="w-full flex items-center justify-between h-8 px-3 rounded-lg border border-warning/50 bg-warning/10 hover:bg-warning/20 text-warning cursor-pointer transition-colors text-xs font-medium select-none shadow-sm"
          title="点击管理撤单（退款）"
        >
          <div className="flex items-center gap-2 min-w-0">
            <Undo2 className="w-3.5 h-3.5 text-warning flex-shrink-0" />
            <span className="whitespace-nowrap">可撤单:</span>
            <span className="font-semibold truncate">
              还剩{" "}
              {typeof retraction.hoursLeft === "number"
                ? retraction.hoursLeft > 48
                  ? `${Math.ceil(retraction.hoursLeft / 24)} 天`
                  : `${retraction.hoursLeft} 小时`
                : retraction.daysLeft ? `${retraction.daysLeft} 天` : "在期内"}
            </span>
          </div>
          <span className="text-[11px] underline flex-shrink-0 ml-2">立即撤单 ↗</span>
        </button>
      )}

      {/* 2. 核心元数据对称网格：移动端 2x2 对称，平板/桌面端 4 列平铺 */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
        {/* Cell 1: 系统 (OS) */}
        {rawOs ? (
          onOsClick ? (
            <button
              type="button"
              onClick={onOsClick}
              className="w-full flex items-center justify-between h-8 px-2.5 rounded-lg border border-border/80 bg-background hover:bg-muted/70 hover:border-primary/40 active:scale-[0.99] cursor-pointer transition-all text-xs select-none group min-w-0"
              title={`${osClickTitle} (当前: ${rawOs})`}
            >
              <div className="flex items-center gap-1.5 min-w-0 flex-1">
                <OsIcon
                  templateName={rawOs}
                  distribution={os?.distribution}
                  family={os?.family}
                  size={15}
                  className="rounded flex-shrink-0"
                />
                <span className="text-muted-foreground text-[11px] whitespace-nowrap">系统:</span>
                <span className="font-medium text-foreground truncate text-xs">
                  {displayOs}
                </span>
              </div>
              <ExternalLink className="w-2.5 h-2.5 text-muted-foreground/50 group-hover:text-primary transition-colors flex-shrink-0 ml-1" />
            </button>
          ) : (
            <div
              className="w-full flex items-center justify-between h-8 px-2.5 rounded-lg border border-border/70 bg-secondary/30 text-xs select-none min-w-0"
              title={`原始镜像: ${rawOs}`}
            >
              <div className="flex items-center gap-1.5 min-w-0 flex-1">
                <OsIcon
                  templateName={rawOs}
                  distribution={os?.distribution}
                  family={os?.family}
                  size={15}
                  className="rounded flex-shrink-0"
                />
                <span className="text-muted-foreground text-[11px] whitespace-nowrap">系统:</span>
                <span className="font-medium text-foreground truncate text-xs">
                  {displayOs}
                </span>
              </div>
            </div>
          )
        ) : (
          <div className="w-full h-8 px-2.5 rounded-lg border border-dashed border-border/40 bg-secondary/10 flex items-center text-muted-foreground text-xs">
            系统未就绪
          </div>
        )}

        {/* Cell 2: 续费策略 */}
        {renewal ? (
          onRenewalClick ? (
            <button
              type="button"
              onClick={onRenewalClick}
              className={cn(
                "w-full flex items-center justify-between h-8 px-2.5 rounded-lg border cursor-pointer transition-all text-xs select-none group min-w-0",
                isDeleteAtExpiration
                  ? "border-amber-500/40 bg-amber-500/10 hover:bg-amber-500/20 text-amber-600 dark:text-amber-400"
                  : "border-border/80 bg-background hover:bg-muted/70 hover:border-primary/40 active:scale-[0.99] text-foreground"
              )}
              title={
                isDeleteAtExpiration
                  ? "此服务器已设定到期注销，届时将被释放销毁。点击可修改续费策略。"
                  : "点击管理续费策略"
              }
            >
              <div className="flex items-center gap-1.5 min-w-0 flex-1">
                {isDeleteAtExpiration ? (
                  <AlertTriangle className="w-3.5 h-3.5 text-amber-500 flex-shrink-0" />
                ) : (
                  <Repeat className="w-3.5 h-3.5 text-muted-foreground flex-shrink-0" />
                )}
                <span
                  className={cn(
                    "text-[11px] whitespace-nowrap",
                    isDeleteAtExpiration
                      ? "text-amber-600 dark:text-amber-400 font-medium"
                      : "text-muted-foreground"
                  )}
                >
                  续费:
                </span>
                <span className={cn("font-medium truncate text-xs", isDeleteAtExpiration && "font-semibold")}>
                  {renewalText}
                </span>
              </div>
              <ExternalLink
                className={cn(
                  "w-2.5 h-2.5 flex-shrink-0 ml-1 transition-colors",
                  isDeleteAtExpiration
                    ? "text-amber-500/70 group-hover:text-amber-500"
                    : "text-muted-foreground/50 group-hover:text-primary"
                )}
              />
            </button>
          ) : (
            <div
              className={cn(
                "w-full flex items-center justify-between h-8 px-2.5 rounded-lg border text-xs select-none min-w-0",
                isDeleteAtExpiration
                  ? "border-amber-500/40 bg-amber-500/10 text-amber-600 dark:text-amber-400"
                  : "border-border/70 bg-secondary/30 text-foreground"
              )}
            >
              <div className="flex items-center gap-1.5 min-w-0 flex-1">
                {isDeleteAtExpiration ? (
                  <AlertTriangle className="w-3.5 h-3.5 text-amber-500 flex-shrink-0" />
                ) : (
                  <Repeat className="w-3.5 h-3.5 text-muted-foreground flex-shrink-0" />
                )}
                <span
                  className={cn(
                    "text-[11px] whitespace-nowrap",
                    isDeleteAtExpiration
                      ? "text-amber-600 dark:text-amber-400"
                      : "text-muted-foreground"
                  )}
                >
                  续费:
                </span>
                <span className="font-medium truncate text-xs">{renewalText}</span>
              </div>
            </div>
          )
        ) : (
          <div className="w-full h-8 px-2.5 rounded-lg border border-dashed border-border/40 bg-secondary/10 flex items-center text-muted-foreground text-xs">
            续费未知
          </div>
        )}

        {/* Cell 3: 到期时间 */}
        {expiration ? (
          <div
            className="w-full flex items-center justify-between h-8 px-2.5 rounded-lg border border-border/70 bg-secondary/30 text-xs select-none min-w-0"
            title={`到期时间: ${new Date(expiration).toLocaleString("zh-CN")}`}
          >
            <div className="flex items-center gap-1.5 min-w-0 flex-1">
              <CalendarClock className="w-3.5 h-3.5 text-muted-foreground flex-shrink-0" />
              <span className="text-muted-foreground text-[11px] whitespace-nowrap">到期:</span>
              <span className="font-mono font-medium text-foreground truncate text-xs">
                {formatDate(expiration)}
              </span>
            </div>
          </div>
        ) : (
          <div className="w-full h-8 px-2.5 rounded-lg border border-dashed border-border/40 bg-secondary/10 flex items-center text-muted-foreground text-xs">
            到期: —
          </div>
        )}

        {/* Cell 4: 开通时间 */}
        {creation ? (
          <div
            className="w-full flex items-center justify-between h-8 px-2.5 rounded-lg border border-border/70 bg-secondary/30 text-xs select-none min-w-0"
            title={`开通时间: ${new Date(creation).toLocaleString("zh-CN")}`}
          >
            <div className="flex items-center gap-1.5 min-w-0 flex-1">
              <CalendarPlus className="w-3.5 h-3.5 text-muted-foreground flex-shrink-0" />
              <span className="text-muted-foreground text-[11px] whitespace-nowrap">开通:</span>
              <span className="font-mono font-medium text-foreground truncate text-xs">
                {formatDate(creation)}
              </span>
            </div>
          </div>
        ) : (
          <div className="w-full h-8 px-2.5 rounded-lg border border-dashed border-border/40 bg-secondary/10 flex items-center text-muted-foreground text-xs">
            开通: —
          </div>
        )}
      </div>
    </div>
  );
}
