import { useState, useEffect, useMemo } from "react";
import { Helmet } from "react-helmet-async";
import {
  CloudLightning, Loader2, Wifi, WifiOff, RefreshCw, Settings2,
  Info, Copy, CheckCircle2, Send, Terminal, Play, Sparkles,
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
import { useAccounts } from "@/hooks/use-accounts";
import { useAvailability, buildAvailabilityMap } from "@/hooks/use-availability";
import { useTelegramPollerStatus } from "@/hooks/use-settings";
import { mergeDcAvailability } from "@/lib/datacenters";
import { cn } from "@/lib/utils";

const TelegramOrderPage = () => {
  const { data: servers } = useServers();
  const { data: accounts } = useAccounts();
  const availQ = useAvailability();
  const availMap = useMemo(() => buildAvailabilityMap(availQ.data), [availQ.data]);

  const [selectedMode, setSelectedMode] = useState<OrderMode['mode']>('buy');
  const [planCode, setPlanCode] = useState('');
  const [datacenter, setDatacenter] = useState('');
  const [quantity, setQuantity] = useState(1);
  const [accountRef, setAccountRef] = useState<string>('');

  // 命令行直接输入
  const [rawInput, setRawInput] = useState<string>('');
  const [isExecutingRaw, setIsExecutingRaw] = useState(false);

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
        toast.success("Bot 命令菜单已注册（/buy /stock /tasks /accounts /subs 等）");
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
    if (item.command?.startsWith("/")) {
      setRawInput(item.command);
    }
    setSelectedMode(item.mode as OrderMode['mode']);
    if (item.planCode) setPlanCode(item.planCode);
    if (item.datacenter) setDatacenter(item.datacenter);
    toast.success("已加载历史命令配置");
  };

  const generateCommand = () => {
    if (selectedMode === 'tasks' || selectedMode === 'accounts' || selectedMode === 'subs') {
      return `/${selectedMode}`;
    }
    let cmd = `/${selectedMode} ${planCode}`;
    if (datacenter && (selectedMode === 'queue' || selectedMode === 'price' || selectedMode === 'buy' || selectedMode === 'watch')) {
      cmd += ` ${datacenter}`;
    }
    if (selectedMode === 'buy' && quantity > 1) {
      cmd += ` ${quantity}`;
    } else if (selectedMode === 'watch' && quantity > 0) {
      cmd += ` x${quantity}`;
    }
    if (accountRef) {
      cmd += ` @${accountRef.replace(/^@/, '')}`;
    }
    return cmd;
  };

  const copyCommand = () => {
    navigator.clipboard.writeText(generateCommand());
    setCopied(true);
    toast.success("命令已复制到剪贴板");
    setTimeout(() => setCopied(false), 2000);
  };

  // 1) 交互命令行直接执行
  const handleExecuteRaw = async (cmdToRun?: string) => {
    const targetCmd = (cmdToRun || rawInput).trim();
    if (!targetCmd) {
      toast.error("请输入要执行的指令");
      return;
    }
    setIsExecutingRaw(true);
    try {
      const result = await api.telegramQuickOrder({
        rawCommand: targetCmd,
      });
      setLastResult(result);
      addToHistory(targetCmd, result.mode || 'cli', '', undefined, result.success);

      if (result.success) {
        toast.success(result.message?.split('\n')[0] || "指令执行成功");
      } else {
        toast.error(result.error || "指令执行失败");
      }
    } catch (error: any) {
      toast.error(`请求失败: ${error.message}`);
      setLastResult({ success: false, error: error.message, command: targetCmd });
      addToHistory(targetCmd, 'cli', '', undefined, false);
    } finally {
      setIsExecutingRaw(false);
    }
  };

  // 2) 结构化表单提交
  const handleSubmit = async () => {
    const isGlobalQuery = selectedMode === 'tasks' || selectedMode === 'accounts' || selectedMode === 'subs';
    if (!isGlobalQuery && !planCode) {
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
        planCode: isGlobalQuery ? undefined : planCode,
        datacenter: datacenter || undefined,
        quantity: selectedMode === 'buy' || selectedMode === 'watch' ? quantity : undefined,
        accountRef: accountRef || undefined,
      });

      setLastResult(result);
      addToHistory(command, selectedMode, planCode, datacenter || undefined, result.success);

      if (result.success) {
        toast.success(result.message?.split('\n')[0] || "操作成功");
      } else {
        toast.error(result.error || "操作失败");
      }
    } catch (error: any) {
      toast.error(`请求失败: ${error.message}`);
      setLastResult({ success: false, error: error.message, command });
      addToHistory(command, selectedMode, planCode, datacenter || undefined, false);
    } finally {
      setIsSubmitting(false);
    }
  };

  const isGlobalMode = selectedMode === 'tasks' || selectedMode === 'accounts' || selectedMode === 'subs';
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

  const quickShortcuts = [
    { label: "⚡ /buy @all 全开抢", cmd: "/buy 24sk102 gra 1 @all" },
    { label: "📦 /stock 查库存", cmd: "/stock 24ska01" },
    { label: "📋 /tasks 抢购队列", cmd: "/tasks" },
    { label: "👤 /accounts 账号矩阵", cmd: "/accounts" },
    { label: "📡 /subs 监控列表", cmd: "/subs" },
    { label: "⏱️ /interval 查看间隔", cmd: "/interval" },
    { label: "🛑 /cancel all 停止全部", cmd: "/cancel all" },
  ];

  return (
    <>
      <Helmet>
        <title>云下单 (Telegram Bot 交互) | OVH Sniper</title>
        <meta name="description" content="云端交互式快捷指令下单与多账户调度" />
      </Helmet>

      <AppLayout>
        <div className="space-y-4 sm:space-y-6">
          <PageHeader
            icon={CloudLightning}
            title="云下单 (Telegram Bot 交互中心)"
            description="全功能云端与 Bot 相同的快捷交互指令 · 多账户调度 (@all / @zone) · 毫秒级任务入队"
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

          {/* 交互命令行直接输入卡片 */}
          <Card className="surface-card rounded-xl border-border bg-gradient-to-r from-secondary/40 via-secondary/20 to-secondary/40 shadow-sm overflow-hidden">
            <CardContent className="p-3 sm:p-4 space-y-2.5">
              <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-2">
                <div className="flex items-center gap-2 flex-1 relative">
                  <div className="absolute left-3 top-1/2 -translate-y-1/2 flex items-center gap-1 text-primary pointer-events-none">
                    <Terminal className="h-4 w-4" />
                    <span className="font-mono text-xs font-bold text-muted-foreground">›</span>
                  </div>
                  <Input
                    value={rawInput}
                    onChange={(e) => setRawInput(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && !e.shiftKey) {
                        e.preventDefault();
                        void handleExecuteRaw();
                      }
                    }}
                    placeholder="键入任意指令，如 /buy 24sk102 gra 1 @all 或 /tasks 或 /accounts 或 24sk102 gra 1..."
                    className="h-9 pl-9 pr-3 text-xs sm:text-sm font-mono bg-background/80 border-border/80 focus-visible:ring-1 focus-visible:ring-primary rounded-lg"
                  />
                </div>
                <Button
                  size="sm"
                  onClick={() => void handleExecuteRaw()}
                  disabled={isExecutingRaw || !rawInput.trim()}
                  className="h-9 px-4 text-xs font-medium gap-1.5 rounded-lg flex-shrink-0 shadow-sm"
                >
                  {isExecutingRaw ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Play className="h-3.5 w-3.5 fill-current" />
                  )}
                  <span>执行指令</span>
                </Button>
              </div>

              {/* 快捷预设指令 Chips */}
              <div className="flex items-center gap-1.5 flex-wrap pt-0.5">
                <span className="text-[10px] text-muted-foreground flex items-center gap-1 mr-1">
                  <Sparkles className="h-3 w-3 text-primary" />
                  <span>快捷预设:</span>
                </span>
                {quickShortcuts.map((sc) => (
                  <button
                    key={sc.cmd}
                    type="button"
                    onClick={() => {
                      setRawInput(sc.cmd);
                      void handleExecuteRaw(sc.cmd);
                    }}
                    className="px-2 py-0.5 rounded-md text-[11px] font-mono border border-border/80 bg-background/60 hover:bg-secondary hover:text-foreground text-muted-foreground transition-colors"
                  >
                    {sc.label}
                  </button>
                ))}
              </div>
            </CardContent>
          </Card>

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

                {isGlobalMode ? (
                  <div className="p-4 rounded-xl border border-dashed border-border/80 bg-secondary/20 text-center space-y-2 py-8">
                    <div className="mx-auto w-8 h-8 rounded-full bg-primary/10 flex items-center justify-center text-primary">
                      {currentMode.icon}
                    </div>
                    <p className="text-xs font-medium text-foreground">全局状态透视指令</p>
                    <p className="text-[11px] text-muted-foreground max-w-xs mx-auto">
                      此模式直接从系统实时获取运行状态，无需指定服务器型号或机房。
                    </p>
                  </div>
                ) : (
                  <>
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

                    {/* 下单账户与多账户路由 */}
                    <div className="space-y-1.5">
                      <div className="flex items-center justify-between">
                        <Label className="text-xs font-medium">下单账户 / 路由策略</Label>
                        <span className="text-[10px] text-muted-foreground font-mono">
                          {accountRef ? (accountRef === "all" ? "@all 全账户" : `@${accountRef}`) : "智能大区推导"}
                        </span>
                      </div>
                      <select
                        value={accountRef}
                        onChange={(e) => setAccountRef(e.target.value)}
                        className="w-full h-8 px-2.5 rounded-lg text-xs bg-background/50 border border-border/80 text-foreground focus:outline-none focus:ring-1 focus:ring-primary"
                      >
                        <option value="">✨ 智能大区推导（自动识别型号所属大区账号）</option>
                        <option value="all">🚀 全账户并发开抢（@all - 全部可用账户同时抢购）</option>
                        {(accounts || []).map((acc, idx) => {
                          const zone = (acc.zone || "EU").toUpperCase();
                          return (
                            <option key={acc.id} value={`${idx + 1}`}>
                              👤 {acc.name}（@{idx + 1} · {zone}区 {acc.isDefault ? "★默认" : ""}）
                            </option>
                          );
                        })}
                      </select>
                    </div>

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
                  </>
                )}

                {(planCode || isGlobalMode) && (
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
                  disabled={isSubmitting || (!isGlobalMode && !planCode)}
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
