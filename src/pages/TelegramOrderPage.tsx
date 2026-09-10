import { AppLayout } from "@/components/layout/AppLayout";
import { PageHeader } from "@/components/common/PageHeader";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
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
  Info,
  Wifi,
  WifiOff,
  RefreshCw,
  History,
  Trash2,
  Play,
  Settings2,
} from "lucide-react";
import { useState, useEffect, useMemo } from "react";
import { cn } from "@/lib/utils";
import { api } from "@/lib/api";
import { useTelegramPollerStatus } from "@/hooks/use-settings";
import { toast } from "sonner";
import { useServers } from "@/hooks/useApi";
import { Badge } from "@/components/ui/badge";
import { DatacenterPicker } from "@/components/common/DatacenterPicker";
import { PlanCodeCombobox } from "@/components/common/PlanCodeCombobox";
import { useAvailability, buildAvailabilityMap } from "@/hooks/use-availability";
import { mergeDcAvailability } from "@/lib/datacenters";


interface OrderMode {
  mode: 'stock' | 'queue' | 'monitor' | 'price' | 'buy';
  name: string;
  description: string;
  icon: React.ReactNode;
  example: string;
  color: string;
}

interface CommandHistory {
  id: string;
  command: string;
  mode: string;
  planCode: string;
  datacenter?: string;
  timestamp: number;
  success: boolean;
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

const HISTORY_STORAGE_KEY = 'telegram_command_history';
const MAX_HISTORY_ITEMS = 20;

const TelegramOrderPage = () => {
  const { data: servers } = useServers();
  const availQ = useAvailability();
  const availMap = useMemo(() => buildAvailabilityMap(availQ.data), [availQ.data]);
  const [selectedMode, setSelectedMode] = useState<OrderMode['mode']>('stock');
  const [planCode, setPlanCode] = useState('');
  const [datacenter, setDatacenter] = useState('');
  const [quantity, setQuantity] = useState(1);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [lastResult, setLastResult] = useState<any>(null);
  const [copied, setCopied] = useState(false);
  
  const poller = useTelegramPollerStatus();

  // Command history
  const [commandHistory, setCommandHistory] = useState<CommandHistory[]>([]);

  const matchedServer = useMemo(
    () => (servers || []).find((s) => s.planCode === planCode),
    [servers, planCode]
  );
  const dcAvailability = useMemo(
    () =>
      mergeDcAvailability(
        matchedServer?.datacenters,
        matchedServer ? availMap[matchedServer.planCode] : undefined
      ),
    [matchedServer, availMap]
  );

  const currentMode = orderModes.find(m => m.mode === selectedMode)!;

  // Load command history from localStorage
  useEffect(() => {
    const saved = localStorage.getItem(HISTORY_STORAGE_KEY);
    if (saved) {
      try {
        setCommandHistory(JSON.parse(saved));
      } catch (e) {
        console.error('Failed to parse command history:', e);
      }
    }
  }, []);

  const handleRegisterCommands = async () => {
    try {
      const result = await api.registerTelegramCommands();
      if (result.success) {
        toast.success("Bot 命令菜单已注册（/buy /stock 等）");
      } else {
        toast.error(result.error || "注册失败");
      }
    } catch (error: any) {
      toast.error(`注册失败: ${error.message}`);
    }
  };

  const saveHistory = (history: CommandHistory[]) => {
    localStorage.setItem(HISTORY_STORAGE_KEY, JSON.stringify(history));
    setCommandHistory(history);
  };

  const addToHistory = (command: string, mode: string, planCode: string, datacenter: string | undefined, success: boolean) => {
    const newItem: CommandHistory = {
      id: Date.now().toString(),
      command,
      mode,
      planCode,
      datacenter,
      timestamp: Date.now(),
      success
    };
    
    const newHistory = [newItem, ...commandHistory].slice(0, MAX_HISTORY_ITEMS);
    saveHistory(newHistory);
  };

  const clearHistory = () => {
    saveHistory([]);
    toast.success("历史记录已清空");
  };

  const repeatCommand = (item: CommandHistory) => {
    setSelectedMode(item.mode as OrderMode['mode']);
    setPlanCode(item.planCode);
    setDatacenter(item.datacenter || '');
    toast.success("已加载历史命令配置");
  };

  const handleSubmit = async () => {
    if (!planCode) {
      toast.error("请选择服务器型号");
      return;
    }

    if ((selectedMode === 'queue' || selectedMode === 'price' || selectedMode === 'buy') && !datacenter) {
      toast.error("此模式需要选择机房");
      return;
    }

    const command = generateCommand();
    setIsSubmitting(true);
    try {
      const result = await api.telegramQuickOrder({
        mode: selectedMode,
        planCode,
        datacenter: datacenter || undefined,
        quantity: selectedMode === 'buy' ? quantity : undefined,
      });
      
      setLastResult(result);
      addToHistory(command, selectedMode, planCode, datacenter || undefined, result.success);
      
      if (result.success) {
        toast.success(result.message || "操作成功");
      } else {
        toast.error(result.error || "操作失败");
      }
    } catch (error: any) {
      toast.error(`请求失败: ${error.message}`);
      setLastResult({ success: false, error: error.message });
      addToHistory(command, selectedMode, planCode, datacenter || undefined, false);
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
  const isPollerConnected = !!poller.data?.running;

  return (
    <>
      <Helmet>
        <title>云下单 | OVH Sniper</title>
        <meta name="description" content="云端快捷指令下单" />
      </Helmet>
      
      <AppLayout>
        <div className="space-y-4 sm:space-y-6">
          {/* Header */}
          <PageHeader
            icon={Zap}
            title="云下单"
            description="网页演练云端与 Bot 相同的快捷命令 · 设置填写 Token 和 Chat ID"
            action={
              <div className="flex items-center gap-2">
                {/* Bot Connection Status */}
                {poller.isFetching && !poller.data ? (
                  <span className="flex items-center gap-1.5 text-xs text-muted-foreground bg-muted/60 px-2.5 h-8 rounded-lg border border-border/60 flex-shrink-0">
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    <span className="hidden sm:inline">检查中</span>
                  </span>
                ) : isPollerConnected ? (
                  <span
                    className="flex items-center gap-1.5 text-xs text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 px-2.5 h-8 rounded-lg border border-emerald-500/25 flex-shrink-0"
                    title={poller.data?.botUsername ? `@${poller.data.botUsername} 轮询正常` : "Bot 轮询正常"}
                  >
                    <Wifi className="h-3.5 w-3.5" />
                    <span>轮询中</span>
                    {poller.data?.botUsername ? (
                      <span className="hidden md:inline font-mono text-[11px] text-muted-foreground">
                        @{poller.data.botUsername}
                      </span>
                    ) : null}
                  </span>
                ) : (
                  <span className="flex items-center gap-1.5 text-xs text-destructive bg-destructive/10 px-2.5 h-8 rounded-lg border border-destructive/20 flex-shrink-0">
                    <WifiOff className="h-3.5 w-3.5" />
                    <span>未连接</span>
                  </span>
                )}
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => void poller.refetch()}
                  disabled={poller.isFetching}
                  className="h-8 w-8 p-0 rounded-lg border-border/80 hover:bg-secondary flex-shrink-0"
                  title="刷新 Bot 状态"
                >
                  <RefreshCw className={cn("h-3.5 w-3.5", poller.isFetching && "animate-spin")} />
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleRegisterCommands}
                  className="h-8 text-xs gap-1.5 border-border/80 hover:bg-secondary px-2.5 sm:px-3 rounded-lg flex-shrink-0"
                  title="向 Telegram 注册 Bot 命令菜单（/buy /stock 等）"
                >
                  <Settings2 className="h-3.5 w-3.5 text-muted-foreground" />
                  <span className="hidden sm:inline">注册命令菜单</span>
                  <span className="sm:hidden">注册菜单</span>
                </Button>
              </div>
            }
          />

          {/* Mode Selection Cards */}
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2.5 sm:gap-3">
            {orderModes.map((mode) => (
              <button
                key={mode.mode}
                onClick={() => setSelectedMode(mode.mode)}
                className={cn(
                  "surface-card rounded-xl p-3 sm:p-4 text-left transition-all duration-200 border border-border/80 hover:border-border",
                  selectedMode === mode.mode 
                    ? "border-primary bg-primary/10 text-foreground" 
                    : "text-muted-foreground hover:text-foreground",
                  mode.mode === 'buy' && "col-span-2 sm:col-span-1"
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

          <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 sm:gap-6">
            {/* Configuration Panel */}
            <Card className="surface-card rounded-xl border-border flex flex-col">
              <CardHeader className="p-4 sm:p-5 pb-3 border-b border-border/40">
                <CardTitle className="text-sm font-semibold flex items-center gap-2">
                  <span className="text-primary">{currentMode.icon}</span>
                  <span>{currentMode.name} 配置</span>
                </CardTitle>
              </CardHeader>
              <CardContent className="p-4 sm:p-5 space-y-4 flex-1">
                {/* Mode Info */}
                <div className="p-3 bg-secondary/40 rounded-xl border border-border/60">
                  <div className="flex items-start gap-2.5">
                    <Info className="h-4 w-4 text-primary mt-0.5 flex-shrink-0" />
                    <div className="min-w-0">
                      <p className="text-xs text-foreground/90 leading-relaxed">{currentMode.description}</p>
                      <p className="text-[11px] text-muted-foreground mt-1 font-mono">
                        示例: {currentMode.example}
                      </p>
                    </div>
                  </div>
                </div>

                <div className="space-y-1.5">
                  <Label className="text-xs font-medium">服务器型号 *</Label>
                  <PlanCodeCombobox
                    value={planCode}
                    onChange={setPlanCode}
                    servers={servers || []}
                    placeholder="输入或搜索型号，例如 24ska01"
                  />
                </div>

                {needsDatacenter && (
                  <div className="space-y-1.5">
                    <Label className="text-xs font-medium">选择机房 *</Label>
                    <DatacenterPicker
                      multiple={false}
                      value={datacenter ? [datacenter] : []}
                      onChange={(codes) => setDatacenter(codes[0] || "")}
                      availability={dcAvailability}
                      disabled={!planCode.trim()}
                      placeholder="请先选择服务器型号，再点选机房。"
                    />
                  </div>
                )}

                {/* Quantity (only for buy mode) */}
                {selectedMode === 'buy' && (
                  <div className="space-y-1.5">
                    <Label className="text-xs font-medium">购买数量</Label>
                    <Input 
                      type="number"
                      min={1}
                      max={10}
                      value={quantity}
                      onChange={(e) => setQuantity(parseInt(e.target.value) || 1)}
                      className="h-8 rounded-lg text-xs bg-background/50 border-border/80"
                    />
                  </div>
                )}

                {/* Generated Command */}
                {planCode && (
                  <div className="space-y-1.5">
                    <Label className="text-xs font-medium">生成的快捷命令</Label>
                    <div className="flex items-center gap-2">
                      <code className="flex-1 px-3 py-2 bg-secondary/40 border border-border/80 rounded-lg font-mono text-xs text-primary truncate">
                        {generateCommand()}
                      </code>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={copyCommand}
                        className="h-8 w-8 p-0 rounded-lg border-border/80 hover:bg-secondary flex-shrink-0"
                        title="复制命令"
                      >
                        {copied ? (
                          <CheckCircle2 className="h-3.5 w-3.5 text-primary" />
                        ) : (
                          <Copy className="h-3.5 w-3.5" />
                        )}
                      </Button>
                    </div>
                  </div>
                )}

                {/* Submit Button */}
                <Button 
                  className="w-full h-9 rounded-lg font-medium text-xs gap-1.5 mt-2" 
                  onClick={handleSubmit} 
                  disabled={isSubmitting || !planCode}
                >
                  {isSubmitting ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Send className="h-3.5 w-3.5" />
                  )}
                  执行 {currentMode.name}
                </Button>
              </CardContent>
            </Card>

            {/* Result Panel */}
            <Card className="surface-card rounded-xl border-border flex flex-col">
              <CardHeader className="p-4 sm:p-5 pb-3 border-b border-border/40">
                <CardTitle className="text-sm font-semibold flex items-center gap-2">
                  <MessageSquare className="w-4 h-4 text-primary" />
                  <span>执行结果</span>
                </CardTitle>
              </CardHeader>
              <CardContent className="p-4 sm:p-5 flex-1 flex flex-col justify-center">
                {lastResult ? (
                  <div className="space-y-3 w-full">
                    <div className={cn(
                      "p-3.5 rounded-xl border",
                      lastResult.success 
                        ? "bg-primary/10 border-primary/30" 
                        : "bg-destructive/10 border-destructive/30"
                    )}>
                      <div className="flex items-center gap-2 mb-1.5">
                        {lastResult.success ? (
                          <CheckCircle2 className="h-4 w-4 text-primary flex-shrink-0" />
                        ) : (
                          <Info className="h-4 w-4 text-destructive flex-shrink-0" />
                        )}
                        <span className={cn(
                          "text-xs font-semibold",
                          lastResult.success ? "text-primary" : "text-destructive"
                        )}>
                          {lastResult.success ? "操作成功" : "操作失败"}
                        </span>
                      </div>
                      <p className="text-xs text-foreground/90 leading-relaxed break-words">
                        {lastResult.message || lastResult.error}
                      </p>
                    </div>

                    {lastResult.price && (
                      <div className="p-3 bg-secondary/30 rounded-xl border border-border/60 flex items-center justify-between">
                        <span className="text-xs text-muted-foreground">价格信息</span>
                        <span className="text-base font-bold font-mono text-foreground">
                          {lastResult.price.prices?.withTax?.toFixed(2) || lastResult.price} €
                        </span>
                      </div>
                    )}

                    {lastResult.orderId && (
                      <div className="p-3 bg-secondary/30 rounded-xl border border-border/60 flex items-center justify-between">
                        <span className="text-xs text-muted-foreground">订单 ID</span>
                        <span className="font-mono text-xs font-semibold text-primary">{lastResult.orderId}</span>
                      </div>
                    )}
                  </div>
                ) : (
                  <div className="text-center py-10 text-muted-foreground">
                    <MessageSquare className="h-10 w-10 mx-auto mb-2.5 opacity-20" />
                    <p className="text-xs">执行操作后结果将显示在这里</p>
                  </div>
                )}
              </CardContent>
            </Card>

            {/* Command History Panel */}
            <Card className="surface-card rounded-xl border-border flex flex-col">
              <CardHeader className="p-4 sm:p-5 pb-3 border-b border-border/40 flex flex-row items-center justify-between">
                <CardTitle className="text-sm font-semibold flex items-center gap-2">
                  <History className="w-4 h-4 text-primary" />
                  <span>命令历史</span>
                </CardTitle>
                {commandHistory.length > 0 && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={clearHistory}
                    className="h-7 px-2 text-xs text-muted-foreground hover:text-destructive gap-1"
                  >
                    <Trash2 className="h-3 w-3" />
                    <span>清空</span>
                  </Button>
                )}
              </CardHeader>
              <CardContent className="p-4 sm:p-5 flex-1">
                {commandHistory.length === 0 ? (
                  <div className="text-center py-10 text-muted-foreground">
                    <History className="h-10 w-10 mx-auto mb-2.5 opacity-20" />
                    <p className="text-xs">暂无历史记录</p>
                  </div>
                ) : (
                  <div className="space-y-2 max-h-80 overflow-y-auto pr-1">
                    {commandHistory.map((item) => (
                      <div 
                        key={item.id}
                        className={cn(
                          "p-2.5 rounded-lg border transition-all hover:border-primary/40 cursor-pointer group bg-secondary/20",
                          item.success ? "border-border/70" : "border-destructive/30"
                        )}
                        onClick={() => repeatCommand(item)}
                      >
                        <div className="flex items-center justify-between gap-2">
                          <code className="font-mono text-xs text-primary truncate">
                            {item.command}
                          </code>
                          <Button 
                            variant="ghost" 
                            size="sm" 
                            className="opacity-0 group-hover:opacity-100 transition-opacity h-6 w-6 p-0 rounded-md"
                          >
                            <Play className="h-3 w-3" />
                          </Button>
                        </div>
                        <div className="flex items-center gap-2 mt-1.5 flex-wrap">
                          <Badge variant="outline" className="text-[10px] px-1.5 py-0 rounded">
                            {orderModes.find(m => m.mode === item.mode)?.name}
                          </Badge>
                          <span className="text-[11px] text-muted-foreground font-mono">
                            {new Date(item.timestamp).toLocaleString("zh-CN")}
                          </span>
                          {!item.success && (
                            <span className="text-[10px] text-destructive font-medium">失败</span>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </div>

          {/* Command Reference */}
          <Card className="surface-card rounded-xl border-border">
            <CardHeader className="p-4 sm:p-5 pb-3 border-b border-border/40">
              <CardTitle className="text-sm font-semibold flex items-center gap-2">
                <Info className="w-4 h-4 text-primary" />
                <span>快捷指令参考与使用说明</span>
              </CardTitle>
            </CardHeader>
            <CardContent className="p-4 sm:p-5 space-y-4">
              {/* Desktop Table View */}
              <div className="hidden md:block overflow-x-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="text-muted-foreground uppercase border-b border-border/60 text-[11px]">
                      <th className="text-left py-2.5 px-3">命令</th>
                      <th className="text-left py-2.5 px-3">格式</th>
                      <th className="text-left py-2.5 px-3">说明</th>
                      <th className="text-left py-2.5 px-3">示例</th>
                    </tr>
                  </thead>
                  <tbody>
                    {[
                      {
                        command: "/buy",
                        format: "/buy [型号] [机房] [数量]",
                        description: "快速下单或抢购排队（有货秒抢 / 缺货挂机）",
                        example: "/buy 24ska01 gra 1",
                        badgeClass: "border-red-500/30 text-red-400 bg-red-500/10",
                      },
                      {
                        command: "/stock",
                        format: "/stock <型号>",
                        description: "查询实时库存并支持直接点选加购",
                        example: "/stock 24ska01",
                        badgeClass: "border-blue-500/30 text-blue-400 bg-blue-500/10",
                      },
                      {
                        command: "/monitor",
                        format: "/monitor <型号> [机房...]",
                        description: "添加型号监控，上架有货时自动发送通知",
                        example: "/monitor 24ska01 gra rbx",
                        badgeClass: "border-emerald-500/30 text-emerald-400 bg-emerald-500/10",
                      },
                      {
                        command: "/price",
                        format: "/price <型号> <机房>",
                        description: "查询指定型号在特定机房的实际落地价格",
                        example: "/price 24ska01 gra",
                        badgeClass: "border-amber-500/30 text-amber-400 bg-amber-500/10",
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
                    ].map((cmd) => (
                      <tr 
                        key={cmd.command}
                        className="border-b border-border/40 hover:bg-secondary/30 transition-colors"
                      >
                        <td className="py-2.5 px-3">
                          <Badge variant="outline" className={cn("font-mono text-xs px-2 py-0.5 rounded-md", cmd.badgeClass)}>
                            {cmd.command}
                          </Badge>
                        </td>
                        <td className="py-2.5 px-3 font-mono text-muted-foreground">
                          {cmd.format}
                        </td>
                        <td className="py-2.5 px-3 text-foreground/90">
                          {cmd.description}
                        </td>
                        <td className="py-2.5 px-3 font-mono text-primary">
                          <div className="flex items-center gap-1.5">
                            <span>{cmd.example}</span>
                            <button
                              type="button"
                              onClick={() => {
                                navigator.clipboard.writeText(cmd.example);
                                toast.success(`已复制: ${cmd.example}`);
                              }}
                              className="text-muted-foreground hover:text-primary transition-colors p-1 rounded hover:bg-secondary"
                              title="复制示例"
                            >
                              <Copy className="h-3 w-3" />
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Mobile Command Cards */}
              <div className="md:hidden grid grid-cols-1 gap-2.5">
                {[
                  {
                    command: "/buy",
                    format: "/buy [型号] [机房] [数量]",
                    description: "快速下单或抢购排队（有货秒抢 / 缺货挂机）",
                    example: "/buy 24ska01 gra 1",
                    badgeClass: "border-red-500/30 text-red-400 bg-red-500/10",
                  },
                  {
                    command: "/stock",
                    format: "/stock <型号>",
                    description: "查询实时库存并支持直接点选加购",
                    example: "/stock 24ska01",
                    badgeClass: "border-blue-500/30 text-blue-400 bg-blue-500/10",
                  },
                  {
                    command: "/monitor",
                    format: "/monitor <型号> [机房...]",
                    description: "添加型号监控，上架有货时自动发送通知",
                    example: "/monitor 24ska01 gra rbx",
                    badgeClass: "border-emerald-500/30 text-emerald-400 bg-emerald-500/10",
                  },
                  {
                    command: "/price",
                    format: "/price <型号> <机房>",
                    description: "查询指定型号在特定机房的实际落地价格",
                    example: "/price 24ska01 gra",
                    badgeClass: "border-amber-500/30 text-amber-400 bg-amber-500/10",
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
                ].map((cmd) => (
                  <div
                    key={cmd.command}
                    className="p-3 rounded-xl border border-border/70 bg-secondary/20 space-y-2"
                  >
                    <div className="flex items-center justify-between">
                      <Badge variant="outline" className={cn("font-mono text-xs px-2 py-0.5 rounded-md", cmd.badgeClass)}>
                        {cmd.command}
                      </Badge>
                      <button
                        type="button"
                        onClick={() => {
                          navigator.clipboard.writeText(cmd.example);
                          toast.success(`已复制: ${cmd.example}`);
                        }}
                        className="inline-flex items-center gap-1 text-[11px] font-mono text-primary bg-primary/10 hover:bg-primary/20 px-2 py-1 rounded-md border border-primary/20 transition-colors"
                        title="复制示例"
                      >
                        <span>{cmd.example}</span>
                        <Copy className="h-3 w-3 ml-0.5" />
                      </button>
                    </div>
                    <p className="text-xs text-foreground/90">{cmd.description}</p>
                    <div className="text-[11px] font-mono text-muted-foreground bg-secondary/50 px-2.5 py-1 rounded-md">
                      格式: {cmd.format}
                    </div>
                  </div>
                ))}
              </div>

              {/* Free-form & Tips */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3 pt-2">
                <div className="p-3.5 bg-secondary/30 rounded-xl border border-border/70 text-xs space-y-2">
                  <div className="flex items-center gap-1.5 font-medium text-foreground">
                    <Zap className="h-3.5 w-3.5 text-amber-500" />
                    <span>免斜杠极速模式 (直接发送)</span>
                  </div>
                  <p className="text-muted-foreground leading-relaxed text-[12px]">
                    在 Telegram 聊天框或快捷命令中直接发送（无需斜杠前缀）：
                  </p>
                  <div className="flex items-center justify-between bg-background/80 px-3 py-2 rounded-lg border border-border/70 font-mono text-xs text-primary">
                    <span>24ska01 gra 1</span>
                    <button
                      type="button"
                      onClick={() => {
                        navigator.clipboard.writeText("24ska01 gra 1");
                        toast.success("已复制: 24ska01 gra 1");
                      }}
                      className="text-muted-foreground hover:text-primary transition-colors ml-2 p-1 rounded hover:bg-secondary"
                      title="复制"
                    >
                      <Copy className="h-3.5 w-3.5" />
                    </button>
                  </div>
                  <p className="text-muted-foreground text-[11px]">
                    格式: <code className="font-mono text-foreground/80 bg-secondary/50 px-1.5 py-0.5 rounded">&lt;型号&gt; [机房] [数量] [系统选项]</code>
                  </p>
                </div>

                <div className="p-3.5 bg-secondary/30 rounded-xl border border-border/70 text-xs space-y-2">
                  <div className="flex items-center gap-1.5 font-medium text-foreground">
                    <Info className="h-3.5 w-3.5 text-primary" />
                    <span>使用与安全说明</span>
                  </div>
                  <ul className="text-muted-foreground space-y-1.5 text-[11px] leading-relaxed list-disc list-inside">
                    <li>
                      <span className="text-foreground/90 font-medium">鉴权安全：</span>
                      仅在【系统设置】中授权的 <code className="font-mono text-primary bg-secondary/50 px-1 py-0.5 rounded">Chat ID</code> 可下单。
                    </li>
                    <li>
                      <span className="text-foreground/90 font-medium">机房代码：</span>
                      支持常用缩写如 <code className="font-mono text-primary bg-secondary/50 px-1 py-0.5 rounded">gra</code> / <code className="font-mono text-primary bg-secondary/50 px-1 py-0.5 rounded">rbx</code> / <code className="font-mono text-primary bg-secondary/50 px-1 py-0.5 rounded">bhs</code> / <code className="font-mono text-primary bg-secondary/50 px-1 py-0.5 rounded">sbg</code> / <code className="font-mono text-primary bg-secondary/50 px-1 py-0.5 rounded">waw</code>（不区分大小写）。
                    </li>
                    <li>
                      <span className="text-foreground/90 font-medium">交互点选：</span>
                      在 Telegram 中发送 <code className="font-mono text-primary bg-secondary/50 px-1 py-0.5 rounded">/start</code> 或 <code className="font-mono text-primary bg-secondary/50 px-1 py-0.5 rounded">/buy</code> 亦可展开多级菜单点选。
                    </li>
                  </ul>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </AppLayout>
    </>
  );
};

export default TelegramOrderPage;
