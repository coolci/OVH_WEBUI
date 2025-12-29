import { AppLayout } from "@/components/layout/AppLayout";
import { TerminalCard } from "@/components/ui/terminal-card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Helmet } from "react-helmet-async";
import { Label } from "@/components/ui/label";
import { 
  MessageSquare, 
  Send, 
  Loader2,
  Search,
  ShoppingCart,
  Eye,
  DollarSign,
  Zap,
  Copy,
  CheckCircle2,
  Info
} from "lucide-react";
import { useState } from "react";
import { cn } from "@/lib/utils";
import { api } from "@/lib/api";
import { toast } from "sonner";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import { useServers } from "@/hooks/useApi";
import { Badge } from "@/components/ui/badge";

interface OrderMode {
  mode: 'stock' | 'queue' | 'monitor' | 'price' | 'buy';
  name: string;
  description: string;
  icon: React.ReactNode;
  example: string;
  color: string;
}

const orderModes: OrderMode[] = [
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
  }
];

const TelegramOrderPage = () => {
  const { data: servers } = useServers();
  const [selectedMode, setSelectedMode] = useState<OrderMode['mode']>('stock');
  const [planCode, setPlanCode] = useState('');
  const [datacenter, setDatacenter] = useState('');
  const [quantity, setQuantity] = useState(1);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [lastResult, setLastResult] = useState<any>(null);
  const [copied, setCopied] = useState(false);

  const datacenterOptions = ["gra", "rbx", "sbg", "bhs", "waw", "lon", "fra", "par"];

  const currentMode = orderModes.find(m => m.mode === selectedMode)!;

  const handleSubmit = async () => {
    if (!planCode) {
      toast.error("请选择服务器型号");
      return;
    }

    if ((selectedMode === 'queue' || selectedMode === 'price' || selectedMode === 'buy') && !datacenter) {
      toast.error("此模式需要选择机房");
      return;
    }

    setIsSubmitting(true);
    try {
      const result = await api.telegramQuickOrder({
        mode: selectedMode,
        planCode,
        datacenter: datacenter || undefined,
        quantity: selectedMode === 'buy' ? quantity : undefined,
      });
      
      setLastResult(result);
      
      if (result.success) {
        toast.success(result.message || "操作成功");
      } else {
        toast.error(result.error || "操作失败");
      }
    } catch (error: any) {
      toast.error(`请求失败: ${error.message}`);
      setLastResult({ success: false, error: error.message });
    } finally {
      setIsSubmitting(false);
    }
  };

  const generateCommand = () => {
    let cmd = `/${selectedMode} ${planCode}`;
    if (datacenter && (selectedMode === 'queue' || selectedMode === 'price' || selectedMode === 'buy')) {
      cmd += ` ${datacenter}`;
    }
    if (selectedMode === 'buy' && quantity > 1) {
      cmd += ` ${quantity}`;
    }
    return cmd;
  };

  const copyCommand = () => {
    navigator.clipboard.writeText(generateCommand());
    setCopied(true);
    toast.success("命令已复制到剪贴板");
    setTimeout(() => setCopied(false), 2000);
  };

  const needsDatacenter = selectedMode === 'queue' || selectedMode === 'price' || selectedMode === 'buy';

  return (
    <>
      <Helmet>
        <title>Telegram 下单 | OVH Sniper</title>
        <meta name="description" content="通过Telegram快速下单" />
      </Helmet>
      
      <AppLayout>
        <div className="space-y-6">
          {/* Header */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold text-primary flex items-center gap-2">
                <span className="text-muted-foreground">&gt;</span>
                Telegram 快速下单
                <span className="cursor-blink">_</span>
              </h1>
              <p className="text-sm text-muted-foreground mt-1">
                通过 Telegram 消息快速执行下单操作，支持5种模式
              </p>
            </div>
          </div>

          {/* Mode Selection Cards */}
          <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
            {orderModes.map((mode) => (
              <button
                key={mode.mode}
                onClick={() => setSelectedMode(mode.mode)}
                className={cn(
                  "terminal-card p-4 text-left transition-all hover:border-primary/50",
                  selectedMode === mode.mode 
                    ? "border-primary bg-primary/10 shadow-glow-sm" 
                    : "border-border"
                )}
              >
                <div className={cn("mb-2", mode.color)}>
                  {mode.icon}
                </div>
                <h3 className="font-medium text-sm">{mode.name}</h3>
                <p className="text-xs text-muted-foreground mt-1 line-clamp-2">
                  {mode.description}
                </p>
              </button>
            ))}
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* Configuration Panel */}
            <TerminalCard
              title={`${currentMode.name} 配置`}
              icon={currentMode.icon}
            >
              <div className="space-y-4">
                {/* Mode Info */}
                <div className="p-3 bg-muted/50 rounded-sm border border-border">
                  <div className="flex items-start gap-2">
                    <Info className="h-4 w-4 text-primary mt-0.5" />
                    <div>
                      <p className="text-sm">{currentMode.description}</p>
                      <p className="text-xs text-muted-foreground mt-1 font-mono">
                        示例: {currentMode.example}
                      </p>
                    </div>
                  </div>
                </div>

                {/* Server Selection */}
                <div className="space-y-2">
                  <Label>服务器型号 *</Label>
                  <Select value={planCode} onValueChange={setPlanCode}>
                    <SelectTrigger>
                      <SelectValue placeholder="选择服务器型号" />
                    </SelectTrigger>
                    <SelectContent>
                      {servers?.map(server => (
                        <SelectItem key={server.planCode} value={server.planCode}>
                          {server.name} ({server.planCode})
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                {/* Datacenter Selection */}
                {needsDatacenter && (
                  <div className="space-y-2">
                    <Label>目标机房 *</Label>
                    <Select value={datacenter} onValueChange={setDatacenter}>
                      <SelectTrigger>
                        <SelectValue placeholder="选择机房" />
                      </SelectTrigger>
                      <SelectContent>
                        {datacenterOptions.map(dc => (
                          <SelectItem key={dc} value={dc}>
                            {dc.toUpperCase()}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                )}

                {/* Quantity (only for buy mode) */}
                {selectedMode === 'buy' && (
                  <div className="space-y-2">
                    <Label>购买数量</Label>
                    <Input 
                      type="number"
                      min={1}
                      max={10}
                      value={quantity}
                      onChange={(e) => setQuantity(parseInt(e.target.value) || 1)}
                    />
                  </div>
                )}

                {/* Generated Command */}
                {planCode && (
                  <div className="space-y-2">
                    <Label>生成的命令</Label>
                    <div className="flex items-center gap-2">
                      <code className="flex-1 px-3 py-2 bg-background border border-border rounded-sm font-mono text-sm text-primary">
                        {generateCommand()}
                      </code>
                      <Button variant="outline" size="sm" onClick={copyCommand}>
                        {copied ? (
                          <CheckCircle2 className="h-4 w-4 text-primary" />
                        ) : (
                          <Copy className="h-4 w-4" />
                        )}
                      </Button>
                    </div>
                    <p className="text-xs text-muted-foreground">
                      可直接发送此命令到 Telegram Bot
                    </p>
                  </div>
                )}

                {/* Submit Button */}
                <Button 
                  className="w-full" 
                  onClick={handleSubmit} 
                  disabled={isSubmitting || !planCode}
                >
                  {isSubmitting ? (
                    <Loader2 className="h-4 w-4 mr-2 animate-spin" />
                  ) : (
                    <Send className="h-4 w-4 mr-2" />
                  )}
                  执行 {currentMode.name}
                </Button>
              </div>
            </TerminalCard>

            {/* Result Panel */}
            <TerminalCard
              title="执行结果"
              icon={<MessageSquare className="h-4 w-4" />}
            >
              {lastResult ? (
                <div className="space-y-4">
                  <div className={cn(
                    "p-4 rounded-sm border",
                    lastResult.success 
                      ? "bg-primary/10 border-primary/30" 
                      : "bg-destructive/10 border-destructive/30"
                  )}>
                    <div className="flex items-center gap-2 mb-2">
                      {lastResult.success ? (
                        <CheckCircle2 className="h-5 w-5 text-primary" />
                      ) : (
                        <Info className="h-5 w-5 text-destructive" />
                      )}
                      <span className={cn(
                        "font-medium",
                        lastResult.success ? "text-primary" : "text-destructive"
                      )}>
                        {lastResult.success ? "操作成功" : "操作失败"}
                      </span>
                    </div>
                    <p className="text-sm">
                      {lastResult.message || lastResult.error}
                    </p>
                  </div>

                  {/* Additional Result Info */}
                  {lastResult.price && (
                    <div className="p-3 bg-muted/50 rounded-sm border border-border">
                      <p className="text-sm text-muted-foreground mb-1">价格信息</p>
                      <p className="text-lg font-bold font-mono text-accent">
                        {lastResult.price.prices?.withTax?.toFixed(2) || lastResult.price} €
                      </p>
                    </div>
                  )}

                  {lastResult.orderId && (
                    <div className="p-3 bg-muted/50 rounded-sm border border-border">
                      <p className="text-sm text-muted-foreground mb-1">订单ID</p>
                      <p className="font-mono text-primary">{lastResult.orderId}</p>
                    </div>
                  )}
                </div>
              ) : (
                <div className="text-center py-12 text-muted-foreground">
                  <MessageSquare className="h-12 w-12 mx-auto mb-4 opacity-30" />
                  <p>执行操作后结果将显示在这里</p>
                </div>
              )}
            </TerminalCard>
          </div>

          {/* Command Reference */}
          <TerminalCard
            title="命令参考"
            icon={<Info className="h-4 w-4" />}
          >
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-xs text-muted-foreground uppercase border-b border-border">
                    <th className="text-left py-3 px-2">命令</th>
                    <th className="text-left py-3 px-2">格式</th>
                    <th className="text-left py-3 px-2">说明</th>
                    <th className="text-left py-3 px-2">示例</th>
                  </tr>
                </thead>
                <tbody>
                  {orderModes.map((mode) => (
                    <tr 
                      key={mode.mode}
                      className="border-b border-border/50 hover:bg-muted/30 transition-colors"
                    >
                      <td className="py-3 px-2">
                        <Badge variant="outline" className={mode.color}>
                          /{mode.mode}
                        </Badge>
                      </td>
                      <td className="py-3 px-2 font-mono text-xs">
                        /{mode.mode} &lt;planCode&gt; [datacenter] [quantity]
                      </td>
                      <td className="py-3 px-2 text-muted-foreground">
                        {mode.description}
                      </td>
                      <td className="py-3 px-2 font-mono text-xs text-primary">
                        {mode.example}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </TerminalCard>
        </div>
      </AppLayout>
    </>
  );
};

export default TelegramOrderPage;
