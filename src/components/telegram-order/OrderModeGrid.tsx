import {
  Search, ShoppingCart, Eye, DollarSign, Zap, Radar,
  ListChecks, Users, Radio,
} from "lucide-react";
import { cn } from "@/lib/utils";
import type { OrderMode } from "./types";

export const ORDER_MODES: (OrderMode & { icon: React.ReactNode })[] = [
  {
    mode: 'buy',
    name: '快速下单',
    description: '立即尝试购买指定服务器',
    icon: <Zap className="h-5 w-5" />,
    example: '/buy 24sk102 gra',
    color: 'text-red-500'
  },
  {
    mode: 'stock',
    name: '库存查询',
    description: '查询指定服务器的实时库存状态',
    icon: <Search className="h-5 w-5" />,
    example: '/stock 24sk102',
    color: 'text-blue-500'
  },
  {
    mode: 'watch',
    name: '盯盘补货',
    description: '缺货自动盯盘，支持补货通知或自动抢购',
    icon: <Radar className="h-5 w-5" />,
    example: '/watch 24sk102 gra x1',
    color: 'text-purple-500'
  },
  {
    mode: 'queue',
    name: '加入队列',
    description: '将服务器添加到抢购队列中',
    icon: <ShoppingCart className="h-5 w-5" />,
    example: '/queue 24sk102 gra',
    color: 'text-orange-500'
  },
  {
    mode: 'tasks',
    name: '挂机任务',
    description: '查看当前正在后台重试抢购的任务队列',
    icon: <ListChecks className="h-5 w-5" />,
    example: '/tasks',
    color: 'text-cyan-500'
  },
  {
    mode: 'accounts',
    name: '账户矩阵',
    description: '查看已绑定账户、大区归属与快捷别名',
    icon: <Users className="h-5 w-5" />,
    example: '/accounts',
    color: 'text-emerald-500'
  },
  {
    mode: 'subs',
    name: '监控清单',
    description: '查看当前正在盯盘监控的服务器清单',
    icon: <Radio className="h-5 w-5" />,
    example: '/subs',
    color: 'text-pink-500'
  },
  {
    mode: 'price',
    name: '价格查询',
    description: '查询服务器在指定机房的价格',
    icon: <DollarSign className="h-5 w-5" />,
    example: '/price 24sk102 gra',
    color: 'text-yellow-500'
  },
  {
    mode: 'monitor',
    name: '添加监控',
    description: '添加服务器到监控列表，有货时通知',
    icon: <Eye className="h-5 w-5" />,
    example: '/monitor 24sk102',
    color: 'text-green-500'
  },
];

interface OrderModeGridProps {
  selectedMode: OrderMode['mode'];
  onSelectMode: (mode: OrderMode['mode']) => void;
}

export function OrderModeGrid({ selectedMode, onSelectMode }: OrderModeGridProps) {
  return (
    <div className="grid grid-cols-3 sm:grid-cols-3 md:grid-cols-5 lg:grid-cols-9 gap-2 sm:gap-2.5">
      {ORDER_MODES.map((mode) => (
        <button
          key={mode.mode}
          onClick={() => onSelectMode(mode.mode)}
          className={cn(
            "surface-card rounded-xl p-2.5 sm:p-3 text-left transition-all duration-200 border border-border/80 hover:border-border flex flex-col justify-between",
            selectedMode === mode.mode
              ? "border-primary bg-primary/10 text-foreground ring-1 ring-primary/40 shadow-sm"
              : "text-muted-foreground hover:text-foreground hover:bg-secondary/40"
          )}
        >
          <div className={cn("mb-1 sm:mb-1.5", mode.color)}>
            {mode.icon}
          </div>
          <div>
            <h3 className="font-semibold text-xs text-foreground truncate">{mode.name}</h3>
            <p className="text-[10px] text-muted-foreground mt-0.5 line-clamp-1 hidden md:block">
              {mode.example}
            </p>
          </div>
        </button>
      ))}
    </div>
  );
}

