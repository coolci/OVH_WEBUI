import { useState, useEffect, useMemo } from "react";
import { Helmet } from "react-helmet-async";
import {
  CloudLightning, Loader2, Wifi, WifiOff, RefreshCw, Settings2,
  Info, Copy, CheckCircle2, Send,
} from "lucide-react";
import { toast } from "sonner";
import { AppLayout } from "@/components/layout/AppLayout";
import { PageHeader } from "@/components/common/PageHeader";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { DatacenterPicker } from "@/components/common/DatacenterPicker";
import { PlanCodeCombobox } from "@/components/common/PlanCodeCombobox";
import { OrderModeGrid, ORDER_MODES } from "@/components/telegram-order/OrderModeGrid";
import { TerminalConsole } from "@/components/telegram-order/TerminalConsole";
import { CommandHistoryPanel } from "@/components/telegram-order/CommandHistoryPanel";
import { CommandReferenceCard } from "@/components/telegram-order/CommandReferenceCard";
import {
  type OrderMode, type CommandHistory,
  HISTORY_STORAGE_KEY, MAX_HISTORY_ITEMS,
} from "@/components/telegram-order/types";
import { api } from "@/lib/api";
import { useServers } from "@/hooks/use-servers";
import { useAvailability, buildAvailabilityMap } from "@/hooks/use-availability";
import { useTelegramPollerStatus } from "@/hooks/use-settings";
import { mergeDcAvailability } from "@/lib/datacenters";
import { cn } from "@/lib/utils";

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
  const [commandHistory, setCommandHistory] = useState<CommandHistory[]>([]);

  const poller = useTelegramPollerStatus();

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

  const currentMode = ORDER_MODES.find(m => m.mode === selectedMode) || ORDER_MODES[0];

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
      success,
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

  const generateCommand = () => {
    let cmd = `/${selectedMode} ${planCode}`;
    if (datacenter && (selectedMode === 'queue' || selectedMode === 'price' || selectedMode === 'buy' || selectedMode === 'watch')) {
      cmd += ` ${datacenter}`;
    }
    if (selectedMode === 'buy' && quantity > 1) {
      cmd += ` ${quantity}`;
    } else if (selectedMode === 'watch' && quantity > 0) {
      cmd += ` x${quantity}`;
    }
    return cmd;
  };

  const copyCommand = () => {
    navigator.clipboard.writeText(generateCommand());
    setCopied(true);
    toast.success("命令已复制到剪贴板");
    setTimeout(() => setCopied(false), 2000);
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
        quantity: selectedMode === 'buy' || selectedMode === 'watch' ? quantity : undefined,
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

  const allowDatacenter = selectedMode === 'queue' || selectedMode === 'price' || selectedMode === 'buy' || selectedMode === 'watch';
  const needsDatacenter = selectedMode === 'queue' || selectedMode === 'price' || selectedMode === 'buy';
  const isPollerConnected = !!(
    poller.data?.running ||
    poller.data?.polling?.running ||
    poller.data?.poller?.running
  );
  const botUsername =
    poller.data?.botUsername ||
    poller.data?.polling?.botUsername ||
    poller.data?.poller?.botUsername;

  return (
    <>
      <Helmet>
        <title>云下单 | OVH Sniper</title>
        <meta name="description" content="云端快捷指令下单" />
      </Helmet>

      <AppLayout>
        <div className="space-y-4 sm:space-y-6">
          <PageHeader
            icon={CloudLightning}
            title="云下单"
            description="网页演练云端与 Bot 相同的快捷命令 · 设置填写 Token 和 Chat ID"
            action={
              <div className="flex items-center gap-2">
                {poller.isFetching && !poller.data ? (
                  <span className="flex items-center gap-1.5 text-xs text-muted-foreground bg-muted/60 px-2.5 h-8 rounded-lg border border-border/60 flex-shrink-0">
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    <span className="hidden sm:inline">检查中</span>
                  </span>
                ) : isPollerConnected ? (
                  <span
                    className="flex items-center gap-1.5 text-xs text-emerald-600 dark:text-emerald-400 bg-emerald-500/10 px-2.5 h-8 rounded-lg border border-emerald-500/25 flex-shrink-0"
                    title={botUsername ? `@${botUsername} 轮询正常` : "Bot 轮询正常"}
                  >
                    <Wifi className="h-3.5 w-3.5" />
                    <span>轮询中</span>
                    {botUsername && (
                      <span className="hidden md:inline font-mono text-[11px] text-muted-foreground">
                        @{botUsername}
                      </span>
                    )}
                  </span>
                ) : (
                  <span className="flex items-center gap-1.5 text-xs text-destructive bg-destructive/10 px-2.5 h-8 rounded-lg border border-destructive/20 flex-shrink-0">
                    <WifiOff className="h-3.5 w-3.5" />
                    <span>未连接</span>
                  </span>
                )}
                <Button
                  size="sm"
                  onClick={() => void poller.refetch()}
                  disabled={poller.isFetching}
                  className="h-8 gap-1.5 px-3 rounded-lg shadow-sm flex-shrink-0 text-xs font-medium"
                  title="刷新 Bot 状态"
                >
                  <RefreshCw className={cn("h-3.5 w-3.5", poller.isFetching && "animate-spin")} />
                  <span>刷新</span>
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

          {/* 模式选择网格 */}
          <OrderModeGrid selectedMode={selectedMode} onSelectMode={setSelectedMode} />

          <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 sm:gap-6">
            {/* 配置面板 */}
            <Card className="surface-card rounded-xl border-border flex flex-col">
              <CardHeader className="p-4 sm:p-5 pb-3 border-b border-border/40">
                <CardTitle className="text-sm font-semibold flex items-center gap-2">
                  <span className="text-primary">{currentMode.icon}</span>
                  <span>{currentMode.name} 配置</span>
                </CardTitle>
              </CardHeader>
              <CardContent className="p-4 sm:p-5 space-y-4 flex-1">
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
                  <div className="flex items-center justify-between">
                    <Label className="text-xs font-medium">服务器型号 *</Label>
                    <div className="flex items-center gap-1 flex-wrap">
                      <span className="text-[10px] text-muted-foreground mr-0.5">常用:</span>
                      {["24sk102", "24sk202", "24sk40-v1", "24sk602"].map((code) => (
                        <button
                          key={code}
                          type="button"
                          onClick={() => setPlanCode(code)}
                          className={cn(
                            "px-1.5 py-0.5 rounded text-[10px] font-mono border transition-colors",
                            planCode === code
                              ? "bg-primary text-primary-foreground border-primary font-bold shadow-sm"
                              : "bg-secondary/60 hover:bg-secondary border-border text-muted-foreground hover:text-foreground"
                          )}
                        >
                          {code}
                        </button>
                      ))}
                    </div>
                  </div>
                  <PlanCodeCombobox
                    value={planCode}
                    onChange={setPlanCode}
                    servers={servers || []}
                    placeholder="输入或搜索型号，例如 24sk102"
                  />
                </div>

                {allowDatacenter && (
                  <div className="space-y-1.5">
                    <div className="flex items-center justify-between">
                      <Label className="text-xs font-medium">
                        选择机房 {needsDatacenter ? "*" : "（可选，不选则盯所有机房）"}
                      </Label>
                      {selectedMode === 'watch' && datacenter && (
                        <button
                          type="button"
                          onClick={() => setDatacenter('')}
                          className="text-[11px] text-muted-foreground hover:text-primary transition-colors"
                        >
                          清除机房限制
                        </button>
                      )}
                    </div>
                    <DatacenterPicker
                      multiple={false}
                      value={datacenter ? [datacenter] : []}
                      onChange={(codes) => setDatacenter(codes[0] || "")}
                      availability={dcAvailability}
                      disabled={!planCode.trim()}
                      placeholder={selectedMode === 'watch' ? "默认盯全部机房，也可点选指定机房。" : "请先选择服务器型号，再点选机房。"}
                    />
                  </div>
                )}

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

                {selectedMode === 'watch' && (
                  <div className="space-y-1.5">
                    <div className="flex items-center justify-between">
                      <Label className="text-xs font-medium">自动抢购数量（可选）</Label>
                      <span className="text-[11px] text-muted-foreground">0 为仅通知，≥1 补货自动下单</span>
                    </div>
                    <Input
                      type="number"
                      min={0}
                      max={10}
                      value={quantity}
                      onChange={(e) => setQuantity(Math.max(0, parseInt(e.target.value) || 0))}
                      placeholder="0 = 仅发通知，1 = 自动抢 1 台"
                      className="h-8 rounded-lg text-xs bg-background/50 border-border/80"
                    />
                  </div>
                )}

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

            {/* 终端控制台模拟器 */}
            <TerminalConsole
              selectedMode={selectedMode}
              lastResult={lastResult}
              currentCommand={generateCommand()}
            />

            {/* 历史记录面板 */}
            <CommandHistoryPanel
              history={commandHistory}
              onClear={clearHistory}
              onRepeat={repeatCommand}
            />
          </div>

          {/* 快捷指令参考手册卡片 */}
          <CommandReferenceCard />
        </div>
      </AppLayout>
    </>
  );
};

export default TelegramOrderPage;
