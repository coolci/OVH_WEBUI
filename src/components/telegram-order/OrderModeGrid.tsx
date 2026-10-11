import {
  Search, ShoppingCart, Eye, DollarSign, Zap, Radar,
} from "lucide-react";
import { cn } from "@/lib/utils";
import type { OrderMode } from "./types";

export const ORDER_MODES: (OrderMode & { icon: React.ReactNode })[] = [
  {
    mode: 'stock',
    name: '库存查询',
    description: '查询指定服务器的实时库存状态',
    icon: <Search className="h-5 w-5" />,
    example: '/stock 24ska01',
    color: 'text-blue-500'
  },
  {
    mode: 'queue',
    name: '加入队列',
    description: '将服务器添加到抢购队列中',
    icon: <ShoppingCart className="h-5 w-5" />,
    example: '/queue 24ska01 gra',
    color: 'text-orange-500'
  },
  {
    mode: 'monitor',
    name: '添加监控',
    description: '添加服务器到监控列表，有货时通知',
    icon: <Eye className="h-5 w-5" />,
    example: '/monitor 24ska01',
    color: 'text-green-500'
  },
  {
    mode: 'price',
    name: '价格查询',
    description: '查询服务器在指定机房的价格',
    icon: <DollarSign className="h-5 w-5" />,
    example: '/price 24ska01 gra',
    color: 'text-yellow-500'
  },
  {
    mode: 'buy',
    name: '快速下单',
    description: '立即尝试购买指定服务器',
    icon: <Zap className="h-5 w-5" />,
    example: '/buy 24ska01 gra',
    color: 'text-red-500'
  },
  {
    mode: 'watch',
    name: '盯盘补货',
    description: '缺货自动盯盘，支持补货通知或自动抢购',
    icon: <Radar className="h-5 w-5" />,
    example: '/watch 24ska01 gra x1',
    color: 'text-purple-500'
  }
];

interface OrderModeGridProps {
  selectedMode: OrderMode['mode'];
  onSelectMode: (mode: OrderMode['mode']) => void;
}

export function OrderModeGrid({ selectedMode, onSelectMode }: OrderModeGridProps) {
  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-2.5 sm:gap-3">
      {ORDER_MODES.map((mode) => (
        <button
          key={mode.mode}
          onClick={() => onSelectMode(mode.mode)}
          className={cn(
            "surface-card rounded-xl p-3 sm:p-4 text-left transition-all duration-200 border border-border/80 hover:border-border",
            selectedMode === mode.mode
              ? "border-primary bg-primary/10 text-foreground"
              : "text-muted-foreground hover:text-foreground"
          )}
        >
          <div className={cn("mb-1.5 sm:mb-2", mode.color)}>
            {mode.icon}
          </div>
          <h3 className="font-medium text-xs sm:text-sm text-foreground">{mode.name}</h3>
          <p className="text-[10px] sm:text-xs text-muted-foreground mt-0.5 sm:mt-1 line-clamp-2 hidden xs:block">
            {mode.description}
          </p>
        </button>
      ))}
    </div>
  );
}
