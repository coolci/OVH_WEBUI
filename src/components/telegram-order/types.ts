export interface OrderMode {
  mode: 'stock' | 'queue' | 'monitor' | 'price' | 'buy' | 'watch';
  name: string;
  description: string;
  example: string;
  color: string;
}

export interface CommandHistory {
  id: string;
  command: string;
  mode: string;
  planCode: string;
  datacenter?: string;
  timestamp: number;
  success: boolean;
}

export const HISTORY_STORAGE_KEY = 'telegram_command_history';
export const MAX_HISTORY_ITEMS = 20;

export const COMMAND_REFERENCE = [
  {
    command: "/buy",
    format: "/buy [型号] [机房] [数量]",
    description: "快速下单或抢购排队（有货秒抢 / 缺货挂机）",
    example: "/buy 24sk102 gra 1",
    badgeClass: "border-red-500/30 text-red-400 bg-red-500/10",
  },
  {
    command: "/stock",
    format: "/stock <型号>",
    description: "查询实时库存并支持直接点选加购",
    example: "/stock 24sk102",
    badgeClass: "border-blue-500/30 text-blue-400 bg-blue-500/10",
  },
  {
    command: "/monitor",
    format: "/monitor <型号> [机房...]",
    description: "添加型号监控，上架有货时自动发送通知",
    example: "/monitor 24sk102 gra rbx",
    badgeClass: "border-emerald-500/30 text-emerald-400 bg-emerald-500/10",
  },
  {
    command: "/price",
    format: "/price <型号> <机房>",
    description: "查询指定型号在特定机房的实际落地价格",
    example: "/price 24sk102 gra",
    badgeClass: "border-amber-500/30 text-amber-400 bg-amber-500/10",
  },
  {
    command: "/watch",
    format: "/watch <型号> [机房...] [x数量]",
    description: "盯盘补货（缺货自动盯盘，带 x1 自动下单，不带仅通知）",
    example: "/watch 24sk102 gra x1",
    badgeClass: "border-indigo-500/30 text-indigo-400 bg-indigo-500/10",
  },
  {
    command: "/tasks",
    format: "/tasks",
    description: "查看当前挂机抢购任务队列，支持一键取消",
    example: "/tasks",
    badgeClass: "border-purple-500/30 text-purple-400 bg-purple-500/10",
  },
  {
    command: "/accounts",
    format: "/accounts",
    description: "查看各区域绑定的 OVH 账户与切换默认号",
    example: "/accounts",
    badgeClass: "border-border text-foreground bg-secondary/50",
  },
];
