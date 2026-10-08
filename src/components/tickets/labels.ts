/**
 * OVH 支持工单枚举字典与视觉展示映射
 */
import { Cpu, Cloud, Globe, HardDrive, ShieldAlert, Wrench, CreditCard, HelpCircle, AlertCircle } from "lucide-react";

export interface LabelOption {
  value: string;
  label: string;
  description?: string;
  icon?: React.ComponentType<{ className?: string }>;
}

/** 工单状态映射 */
export const TICKET_STATES: Record<
  string,
  {
    label: string;
    variant: "open" | "closed" | "unknown";
    badgeClass: string;
    dotClass: string;
    glowClass: string;
  }
> = {
  open: {
    label: "处理中",
    variant: "open",
    badgeClass:
      "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/25 shadow-[0_0_12px_rgba(16,185,129,0.12)]",
    dotClass: "bg-emerald-500 animate-pulse",
    glowClass: "from-emerald-500/20 to-teal-500/10",
  },
  closed: {
    label: "已关闭",
    variant: "closed",
    badgeClass:
      "bg-muted/70 text-muted-foreground border-border/60",
    dotClass: "bg-muted-foreground/60",
    glowClass: "from-muted/20 to-transparent",
  },
  unknown: {
    label: "未知状态",
    variant: "unknown",
    badgeClass:
      "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/25",
    dotClass: "bg-amber-500",
    glowClass: "from-amber-500/20 to-transparent",
  },
};

/** 工单主分类 (Category) */
export const TICKET_CATEGORIES: LabelOption[] = [
  {
    value: "assistance",
    label: "技术支持 / 咨询",
    description: "配置故障、系统与网络咨询、日常排障",
    icon: HelpCircle,
  },
  {
    value: "billing",
    label: "财务 / 账单与退款",
    description: "订单咨询、发票账单、扣费疑问、退单申请",
    icon: CreditCard,
  },
  {
    value: "incident",
    label: "突发故障 / 紧急报障",
    description: "硬件离线、宿主机故障、网络中断、IP 被拉黑",
    icon: ShieldAlert,
  },
];

/** 工单子分类 (Subcategory) */
export const TICKET_SUBCATEGORIES: LabelOption[] = [
  { value: "down", label: "服务中断 / 宕机离线 (Down)" },
  { value: "perfs", label: "性能与网络路由异常 (Performance)" },
  { value: "alerts", label: "系统监控告警处理 (Alerts)" },
  { value: "bill", label: "账单发票与退单处理 (Invoice)" },
  { value: "autorenew", label: "自动续费与扣款疑问 (Auto-Renew)" },
  { value: "new", label: "新购方案咨询 (New Order)" },
  { value: "start", label: "新机交付 / 系统初始化 (Start)" },
  { value: "usage", label: "控制台日常操作使用 (Usage)" },
  { value: "inProgress", label: "进行中维护事项 (In Progress)" },
  { value: "other", label: "其他问题 (Other)" },
];

/** 产品类型 (Product) */
export const TICKET_PRODUCTS: LabelOption[] = [
  { value: "dedicated", label: "独立服务器 (Dedicated)", icon: Cpu },
  { value: "vps", label: "VPS 主机 (VPS)", icon: Cloud },
  { value: "publiccloud", label: "公有云 (Public Cloud)", icon: Cloud },
  { value: "iaas", label: "基础设施 (IaaS)", icon: Cpu },
  { value: "housing", label: "机房托管 (Housing)", icon: HardDrive },
  { value: "hosting", label: "虚拟主机 (Web Hosting)", icon: Globe },
  { value: "domain", label: "域名服务 (Domain)", icon: Globe },
  { value: "network", label: "网络与 VRack 带宽", icon: Wrench },
  { value: "storage", label: "存储与备份 (Storage)", icon: HardDrive },
  { value: "mail", label: "企业邮箱 (Email)", icon: Globe },
  { value: "ssl", label: "SSL 安全证书", icon: ShieldAlert },
  { value: "voip", label: "VoIP 通信电话", icon: HelpCircle },
  { value: "cdn", label: "CDN 加速网络", icon: Globe },
];

export function getCategoryLabel(val?: string): string {
  if (!val) return "通用支持";
  const item = TICKET_CATEGORIES.find((c) => c.value === val);
  return item ? item.label : val;
}

export function getSubcategoryLabel(val?: string): string {
  if (!val) return "";
  const item = TICKET_SUBCATEGORIES.find((c) => c.value === val);
  return item ? item.label : val;
}

export function getProductLabel(val?: string): string {
  if (!val) return "通用服务";
  const item = TICKET_PRODUCTS.find((p) => p.value === val);
  return item ? item.label : val;
}

export function getStateMeta(state?: string) {
  if (!state) return TICKET_STATES.unknown;
  return TICKET_STATES[state.toLowerCase()] || {
    label: state,
    variant: "unknown",
    badgeClass: "bg-muted/70 text-muted-foreground border-border/60",
    dotClass: "bg-muted-foreground",
    glowClass: "from-muted/20 to-transparent",
  };
}
