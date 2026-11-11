import { Search, Filter, X, ArrowUpDown, ArrowUpNarrowWide, ArrowDownWideNarrow } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { subsidiaryLabel, type OvhSubsidiary } from "@/lib/ovh-subsidiaries";
import { cn } from "@/lib/utils";

interface ServerFilterBarProps {
  search: string;
  onSearchChange: (value: string) => void;
  onlyAvailable: boolean;
  onToggleOnlyAvailable: () => void;
  priceSort: "default" | "asc" | "desc";
  onPriceSortChange: () => void;
  subsidiary: string;
  onSubsidiaryChange: (value: string) => void;
  /** 账户实际下单用的子公司(配置的 zone) */
  accountSub?: string;
  /** OVH /me 报的 ovhSubsidiary,与 accountSub 不一致时提示 */
  meSub?: string;
  /** 该账户所在大区可选的子公司(三个大区目录互不相通,别的大区选了也对不上列表) */
  subsidiaryOptions: OvhSubsidiary[];
  onResetSubsidiary: () => void;
  totalCount: number;
  isLoading: boolean;
}

export function ServerFilterBar({
  search,
  onSearchChange,
  onlyAvailable,
  onToggleOnlyAvailable,
  priceSort,
  onPriceSortChange,
  subsidiary,
  onSubsidiaryChange,
  accountSub,
  meSub,
  subsidiaryOptions,
  onResetSubsidiary,
  totalCount,
  isLoading,
}: ServerFilterBarProps) {
  const { t } = useTranslation();
  return (
    <Card className="surface-card rounded-xl border-border">
      <CardContent className="p-3.5 sm:p-4 flex flex-col gap-2.5">
        {/* 搜索栏 */}
        <div className="relative">
          <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-muted-foreground pointer-events-none" />
          <Input
            placeholder="搜索 planCode / 型号 / CPU / 内存..."
            value={search}
            onChange={(e) => onSearchChange(e.target.value)}
            className="pl-9 pr-9 rounded-lg h-8 text-xs bg-background/50 border-border/80 focus-visible:ring-1 focus-visible:ring-primary/40"
          />
          {search && (
            <button
              type="button"
              className="absolute right-3 top-1/2 -translate-y-1/2 rounded-md p-0.5 text-muted-foreground hover:text-foreground transition-colors"
              onClick={() => onSearchChange("")}
              aria-label="清空搜索"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          )}
        </div>

        {/* 第二行：筛选 + 地区 + 数量 */}
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5 pt-1 border-t border-border/40 sm:border-t-0 sm:pt-0">
          <div className="flex items-center gap-2 flex-1 min-w-0 flex-wrap sm:flex-nowrap">
            <Button
              variant={onlyAvailable ? "default" : "outline"}
              size="sm"
              className={cn(
                "rounded-lg h-8 text-xs gap-1.5 transition-all font-medium flex-shrink-0",
                onlyAvailable
                  ? ""
                  : "border-border/80 bg-secondary/30 hover:bg-secondary text-foreground"
              )}
              onClick={onToggleOnlyAvailable}
            >
              <Filter className="w-3 h-3" />
              仅显示可用
            </Button>

            <Button
              variant={priceSort === "default" ? "outline" : "default"}
              size="sm"
              className={cn(
                "rounded-lg h-8 text-xs gap-1.5 transition-all font-medium flex-shrink-0",
                priceSort === "default"
                  ? "border-border/80 bg-secondary/30 hover:bg-secondary text-foreground"
                  : ""
              )}
              onClick={onPriceSortChange}
              title={t("servers.sortPriceTitle")}
            >
              {priceSort === "asc" ? (
                <ArrowUpNarrowWide className="w-3.5 h-3.5" />
              ) : priceSort === "desc" ? (
                <ArrowDownWideNarrow className="w-3.5 h-3.5" />
              ) : (
                <ArrowUpDown className="w-3.5 h-3.5" />
              )}
              {priceSort === "default"
                ? t("servers.sortPrice")
                : priceSort === "asc"
                ? t("servers.sortPriceAsc")
                : t("servers.sortPriceDesc")}
            </Button>

            {/* 价格地区选择 */}
            <div className="flex items-center gap-1.5 flex-1 min-w-[150px]">
              <Select value={subsidiary} onValueChange={onSubsidiaryChange}>
                <SelectTrigger
                  className="h-8 rounded-lg text-xs border-border/80 bg-background/50 focus:ring-1 focus:ring-primary/40 w-full sm:w-auto sm:min-w-[140px] sm:max-w-[220px]"
                  title={
                    accountSub
                      ? `价格地区。账户当前绑定 ${accountSub}，实际下单按账户结算`
                      : "切换价格地区"
                  }
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {subsidiaryOptions.map((s) => (
                    <SelectItem key={s.code} value={s.code}>
                      <span className="font-medium">{s.code}</span>
                      <span className="text-muted-foreground ml-1">
                        · {subsidiaryLabel(s.code)}
                        {accountSub === s.code ? " · 我的账户" : ""}
                      </span>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {accountSub && subsidiary !== accountSub && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-8 rounded-full text-xs text-muted-foreground hover:text-foreground px-2 flex-shrink-0"
                  onClick={onResetSubsidiary}
                  title={`回到账户绑定的子公司 ${accountSub}`}
                >
                  ↩ {accountSub}
                </Button>
              )}
              {accountSub && meSub && meSub !== accountSub && (
                <span
                  className="text-[11px] text-warning whitespace-nowrap"
                  title={`账户里配置的子公司是 ${accountSub}(服务器列表、价格、下单都按它),但 OVH 显示这个账户实际归属 ${meSub}。两者不一致时价格和可售机型可能对不上,建议到「设置 → 账户」把子公司改成 ${meSub}`}
                >
                  OVH 归属 {meSub}
                </span>
              )}
            </div>
          </div>
          <span className="text-[12px] text-muted-foreground whitespace-nowrap self-end sm:self-center">
            {isLoading ? "加载中..." : `共 ${totalCount} 款`}
          </span>
        </div>
      </CardContent>
    </Card>
  );
}
