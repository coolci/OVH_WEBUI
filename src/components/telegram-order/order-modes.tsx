import {
  Search, ShoppingCart, Eye, DollarSign, Zap, Radar,
  ListChecks, Users, Radio,
} from "lucide-react";
import type { OrderMode } from "./types";

/** Telegram 云下单的各个模式(命令、说明、示例、图标) */
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
