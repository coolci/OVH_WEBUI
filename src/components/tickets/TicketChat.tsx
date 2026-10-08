import { useState, useRef, useEffect } from "react";
import {
  type SupportTicket,
  useSupportMessages,
  useReplyTicket,
  useCloseTicket,
  useReopenTicket,
} from "@/hooks/ovh/use-tickets";
import { formatDateTime } from "@/lib/format-os";
import { getStateMeta, getProductLabel, getCategoryLabel } from "./labels";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import {
  ArrowLeft,
  Send,
  RefreshCw,
  XCircle,
  RotateCcw,
  Headphones,
  User,
  AlertCircle,
  Clock,
  ShieldCheck,
  Server,
  Layers,
  Sparkles,
} from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Skeleton } from "@/components/common/Skeleton";
import { EmptyState } from "@/components/common/EmptyState";
import { cn } from "@/lib/utils";

interface TicketChatProps {
  ticket: SupportTicket;
  activeAccount?: string;
  onBack?: () => void;
  onRefresh?: () => void;
}

export function TicketChat({ ticket, activeAccount, onBack, onRefresh }: TicketChatProps) {
  const [replyText, setReplyText] = useState("");
  const [closeDialogOpen, setCloseDialogOpen] = useState(false);
  const [reopenDialogOpen, setReopenDialogOpen] = useState(false);
  const [reopenReason, setReopenReason] = useState("");

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const stickToBottomRef = useRef(true);

  const {
    data: messages = [],
    isLoading: isMessagesLoading,
    isFetching: isMessagesFetching,
    refetch: refetchMessages,
  } = useSupportMessages(ticket.ticketId, activeAccount);

  const replyMutation = useReplyTicket(ticket.ticketId, activeAccount);
  const closeMutation = useCloseTicket(ticket.ticketId, activeAccount);
  const reopenMutation = useReopenTicket(ticket.ticketId, activeAccount);

  // 监听用户滚动位置：留有 150px 余量
  const handleScroll = () => {
    const el = scrollContainerRef.current;
    if (!el) return;
    const offsetFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
    stickToBottomRef.current = offsetFromBottom < 150;
  };

  useEffect(() => {
    stickToBottomRef.current = true;
    scrollToBottom(false);
  }, [ticket.ticketId]);

  useEffect(() => {
    if (stickToBottomRef.current) {
      scrollToBottom(true);
    }
  }, [messages.length]);

  const scrollToBottom = (smooth = true) => {
    if (messagesEndRef.current) {
      messagesEndRef.current.scrollIntoView({
        behavior: smooth ? "smooth" : "auto",
        block: "end",
      });
    }
  };

  const handleSendReply = async () => {
    const text = replyText.trim();
    if (!text || replyMutation.isPending) return;

    try {
      await replyMutation.mutateAsync(text);
      setReplyText("");
      stickToBottomRef.current = true;
      setTimeout(() => scrollToBottom(true), 120);
    } catch {
      // hook 统一提示
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSendReply();
    }
  };

  const handleConfirmClose = async () => {
    try {
      await closeMutation.mutateAsync();
      setCloseDialogOpen(false);
      onRefresh?.();
    } catch {
      // handled
    }
  };

  const handleConfirmReopen = async () => {
    const reason = reopenReason.trim();
    if (!reason || reopenMutation.isPending) return;
    try {
      await reopenMutation.mutateAsync(reason);
      setReopenDialogOpen(false);
      setReopenReason("");
      onRefresh?.();
    } catch {
      // handled
    }
  };

  const stateMeta = getStateMeta(ticket.state);
  const isOpen = ticket.state === "open";

  return (
    <div className="flex h-full min-h-0 flex-col bg-card/40 border border-border/70 rounded-2xl shadow-[0_4px_24px_-4px_rgba(0,0,0,0.12)] overflow-hidden backdrop-blur-xl">
      {/* ── 顶部 Header ── */}
      <div className="flex-none px-4 py-3 sm:px-6 sm:py-3.5 border-b border-border/60 bg-card/75 backdrop-blur-xl flex items-center justify-between gap-3">
        <div className="flex items-center gap-3 min-w-0">
          {onBack && (
            <Button
              variant="ghost"
              size="icon"
              onClick={onBack}
              className="lg:hidden h-8 w-8 -ml-1 text-muted-foreground hover:text-foreground shrink-0 rounded-lg"
              title="返回工单列表"
            >
              <ArrowLeft className="h-4 w-4" />
            </Button>
          )}

          <div className="min-w-0 space-y-1">
            <div className="flex items-center gap-2 flex-wrap">
              <span className="font-mono text-xs font-semibold px-2 py-0.5 rounded-md bg-secondary text-secondary-foreground border border-border/60 shadow-xs">
                #{ticket.ticketNumber || ticket.ticketId}
              </span>

              <span
                className={cn(
                  "inline-flex items-center gap-1.5 text-xs px-2.5 py-0.5 rounded-md border font-medium",
                  stateMeta.badgeClass
                )}
              >
                <span className={cn("h-1.5 w-1.5 rounded-full", stateMeta.dotClass)} />
                {stateMeta.label}
              </span>

              {ticket.product && (
                <span className="text-[11px] font-medium px-2 py-0.5 rounded-md bg-muted/60 text-muted-foreground border border-border/40 hidden sm:inline-flex items-center gap-1">
                  <Layers className="h-3 w-3 opacity-60" />
                  {getProductLabel(ticket.product)}
                </span>
              )}

              {ticket.serviceName && (
                <span className="text-[11px] font-mono bg-muted/80 text-foreground/80 px-2 py-0.5 rounded-md border border-border/50 hidden md:inline-flex items-center gap-1 truncate max-w-[220px]">
                  <Server className="h-3 w-3 opacity-60 shrink-0" />
                  {ticket.serviceName}
                </span>
              )}
            </div>

            <h2 className="text-sm sm:text-base font-semibold text-foreground tracking-tight truncate">
              {ticket.subject}
            </h2>
          </div>
        </div>

        {/* 右侧动作按钮 */}
        <div className="flex items-center gap-2 shrink-0">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => {
              refetchMessages();
              onRefresh?.();
            }}
            disabled={isMessagesFetching}
            className="h-8 w-8 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted/80 transition-all"
            title="刷新对话记录"
          >
            <RefreshCw
              className={cn("h-4 w-4", isMessagesFetching && "animate-spin text-primary")}
            />
          </Button>

          {isOpen && ticket.canBeClosed && (
            <Button
              variant="soft-destructive"
              size="sm"
              onClick={() => setCloseDialogOpen(true)}
              className="h-8 px-3 text-xs"
            >
              <XCircle className="h-3.5 w-3.5 mr-1" />
              <span className="hidden sm:inline">关闭工单</span>
              <span className="sm:hidden">关闭</span>
            </Button>
          )}

          {!isOpen && (
            <Button
              variant="soft"
              size="sm"
              onClick={() => setReopenDialogOpen(true)}
              className="h-8 px-3 text-xs"
            >
              <RotateCcw className="h-3.5 w-3.5 mr-1" />
              <span className="hidden sm:inline">重新激活</span>
              <span className="sm:hidden">重开</span>
            </Button>
          )}
        </div>
      </div>

      {/* ── 工单元信息横幅 ── */}
      <div className="flex-none px-4 py-2 sm:px-6 bg-muted/25 border-b border-border/40 text-[11px] text-muted-foreground flex items-center justify-between gap-4 flex-wrap">
        <div className="flex items-center gap-3 flex-wrap">
          <span>问题分类：<strong className="text-foreground/80 font-medium">{getCategoryLabel(ticket.category)}</strong></span>
          {ticket.serviceName && (
            <span className="md:hidden font-mono">服务：{ticket.serviceName}</span>
          )}
          <span>创建时间：{formatDateTime(ticket.creationDate)}</span>
        </div>
        <div className="flex items-center gap-1.5 font-mono">
          <Clock className="h-3 w-3 opacity-70" />
          <span>最新交互：{formatDateTime(ticket.updateDate)}</span>
        </div>
      </div>

      {/* ── 消息流视窗 (彻底告别贴边：宽敞留白 + 微信正规气泡比例 + 专属头像) ── */}
      <div
        ref={scrollContainerRef}
        onScroll={handleScroll}
        className="flex-1 overflow-y-auto px-5 py-6 sm:px-8 md:px-12 lg:px-14 space-y-7 bg-background/30"
      >
        {isMessagesLoading ? (
          <div className="space-y-5 max-w-xl mx-auto py-8">
            <Skeleton className="h-16 w-3/4 rounded-2xl" />
            <Skeleton className="h-16 w-2/3 ml-auto rounded-2xl" />
            <Skeleton className="h-20 w-4/5 rounded-2xl" />
          </div>
        ) : messages.length === 0 ? (
          <div className="py-16">
            <EmptyState
              icon={Headphones}
              title="暂无沟通记录"
              description="该工单尚未生成任何交互消息，官方技术团队受理后将在此回复。"
            />
          </div>
        ) : (
          messages.map((msg, idx) => {
            const isMe = msg.from === "customer";
            const prevMsg = idx > 0 ? messages[idx - 1] : null;
            // 微信风格：当与上一条消息相差超30分钟时，展示居中时间胶囊
            const showTimeDivider =
              !prevMsg ||
              Math.abs(new Date(msg.creationDate).getTime() - new Date(prevMsg.creationDate).getTime()) >
                30 * 60 * 1000;

            return (
              <div key={msg.messageId} className="space-y-3 matrix-fade-in">
                {/* 微信式居中时间胶囊 */}
                {showTimeDivider && (
                  <div className="flex items-center justify-center py-1">
                    <span className="font-mono text-[11px] text-muted-foreground/80 bg-muted/50 border border-border/40 px-3 py-0.5 rounded-full shadow-2xs">
                      {formatDateTime(msg.creationDate)}
                    </span>
                  </div>
                )}

                <div
                  className={cn(
                    "flex items-start gap-3.5 sm:gap-4 w-full group",
                    isMe ? "flex-row-reverse" : "flex-row"
                  )}
                >
                  {/* ── 专属定制高清头像 ── */}
                  <Avatar className="h-10 w-10 sm:h-11 sm:w-11 shrink-0 rounded-2xl ring-1 shadow-md transition-all duration-200 overflow-hidden ring-border/80">
                    {isMe ? (
                      <>
                        <AvatarImage src="/avatars/customer.jpg" alt="客户" className="object-cover" />
                        <AvatarFallback className="bg-emerald-500/20 text-emerald-500 font-semibold text-xs">
                          <User className="h-5 w-5" />
                        </AvatarFallback>
                      </>
                    ) : (
                      <>
                        <AvatarImage src="/avatars/ovh-support.jpg" alt="OVH客服" className="object-cover" />
                        <AvatarFallback className="bg-blue-500/20 text-blue-500 font-semibold text-xs">
                          <Headphones className="h-5 w-5" />
                        </AvatarFallback>
                      </>
                    )}
                  </Avatar>

                  {/* ── 消息主体 (微信比例：限制最大宽度 65%~75%，留出大片舒适空间) ── */}
                  <div
                    className={cn(
                      "flex flex-col max-w-[82%] sm:max-w-[70%] md:max-w-[65%]",
                      isMe ? "items-end" : "items-start"
                    )}
                  >
                    {/* 昵称标签 */}
                    <div
                      className={cn(
                        "flex items-center gap-1.5 mb-1.5 px-1 text-[11px] text-muted-foreground font-medium",
                        isMe ? "flex-row-reverse" : "flex-row"
                      )}
                    >
                      {isMe ? (
                        <span>我 (客户)</span>
                      ) : (
                        <span className="flex items-center gap-1 text-foreground/80 font-semibold">
                          <span>OVH 官方技术支持</span>
                          <ShieldCheck className="h-3.5 w-3.5 text-blue-500" />
                        </span>
                      )}
                    </div>

                    {/* 微信式气泡卡片 (包含自然内边距与舒适的圆角小微尖) */}
                    <div
                      className={cn(
                        "relative px-4.5 py-3 sm:px-5 sm:py-3.5 rounded-2xl text-[13.5px] sm:text-[14px] leading-relaxed whitespace-pre-wrap break-words transition-all duration-200",
                        isMe
                          ? "bg-[#07c160] dark:bg-[#07c160] text-white rounded-tr-xs shadow-[0_4px_18px_rgba(7,193,96,0.22)] border border-[#06b057]/40 selection:bg-emerald-900"
                          : "bg-card/95 dark:bg-[#1f2023] border border-border/80 text-foreground rounded-tl-xs shadow-[0_4px_16px_rgba(0,0,0,0.06)] dark:shadow-none"
                      )}
                    >
                      {msg.body}
                    </div>
                  </div>
                </div>
              </div>
            );
          })
        )}
        <div ref={messagesEndRef} className="h-1" />
      </div>

      {/* ── 底部回复输入区 (带有充足外边距与优雅内缩进) ── */}
      <div className="flex-none p-4 sm:p-5 md:p-6 bg-card/85 border-t border-border/60 backdrop-blur-xl safe-area-bottom">
        {!isOpen ? (
          <div className="flex flex-col sm:flex-row items-center justify-between gap-3 p-4 rounded-xl bg-muted/40 border border-border/60 text-xs">
            <div className="flex items-center gap-2.5 text-muted-foreground text-center sm:text-left">
              <AlertCircle className="h-4.5 w-4.5 text-amber-500 shrink-0" />
              <span>
                当前工单已处理完成并处于关闭状态。如需继续与官方工程师沟通，请点击重新激活。
              </span>
            </div>
            <Button
              size="sm"
              variant="soft"
              onClick={() => setReopenDialogOpen(true)}
              className="shrink-0 h-8.5 px-3.5 text-xs shadow-xs"
            >
              <RotateCcw className="h-3.5 w-3.5 mr-1.5" />
              重新激活此工单
            </Button>
          </div>
        ) : (
          <div className="space-y-2.5">
            <div className="relative rounded-2xl border border-border/70 bg-background/70 hover:border-border focus-within:border-primary/50 focus-within:ring-2 focus-within:ring-primary/20 backdrop-blur-lg shadow-sm transition-all duration-200">
              <Textarea
                value={replyText}
                onChange={(e) => setReplyText(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="在此输入给 OVH 官方技术支持的回复... (桌面端按 Enter 直接发送，Shift + Enter 换行)"
                rows={3}
                autoComplete="off"
                className="resize-none border-0 bg-transparent px-4 py-3 sm:px-4.5 sm:py-3.5 pr-24 text-xs sm:text-sm leading-relaxed focus-visible:ring-0 shadow-none placeholder:text-muted-foreground/60"
              />

              <div className="absolute right-3 bottom-3">
                <Button
                  size="sm"
                  onClick={handleSendReply}
                  disabled={!replyText.trim() || replyMutation.isPending}
                  className="h-8.5 px-4 rounded-xl bg-[#07c160] hover:bg-[#06ad56] text-white shadow-sm hover:shadow transition-all font-medium text-xs disabled:opacity-50"
                >
                  {replyMutation.isPending ? (
                    <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <>
                      <Send className="h-3.5 w-3.5 mr-1.5" />
                      <span>发送回复</span>
                    </>
                  )}
                </Button>
              </div>
            </div>

            <div className="flex items-center justify-between text-[11px] text-muted-foreground px-2">
              <span className="flex items-center gap-1.5">
                <kbd className="px-1.5 py-0.5 rounded border border-border/50 bg-muted/60 font-mono text-[10px]">
                  Enter
                </kbd>
                <span>发送</span>
                <span className="opacity-40">·</span>
                <kbd className="px-1.5 py-0.5 rounded border border-border/50 bg-muted/60 font-mono text-[10px]">
                  Shift + Enter
                </kbd>
                <span>换行</span>
              </span>
              <span className="font-mono">{replyText.length} 字符</span>
            </div>
          </div>
        )}
      </div>

      {/* ── 关闭工单确认弹窗 ── */}
      <Dialog open={closeDialogOpen} onOpenChange={setCloseDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-destructive">
              <XCircle className="h-5 w-5" />
              确认关闭工单 #{ticket.ticketNumber || ticket.ticketId}
            </DialogTitle>
            <DialogDescription className="leading-relaxed pt-1">
              关闭后，OVH 官方技术支持将认为此问题已顺利解决并结单。若后续仍有问题可再次重新激活。确认关闭吗？
            </DialogDescription>
          </DialogHeader>
          <DialogFooter className="gap-2 sm:gap-0 pt-2">
            <Button
              variant="outline"
              onClick={() => setCloseDialogOpen(false)}
              disabled={closeMutation.isPending}
            >
              取消
            </Button>
            <Button
              variant="destructive"
              onClick={handleConfirmClose}
              disabled={closeMutation.isPending}
            >
              {closeMutation.isPending ? "关闭中..." : "确认关闭工单"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ── 重新打开工单弹窗 ── */}
      <Dialog open={reopenDialogOpen} onOpenChange={setReopenDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-primary">
              <RotateCcw className="h-5 w-5" />
              重新激活工单 #{ticket.ticketNumber || ticket.ticketId}
            </DialogTitle>
            <DialogDescription className="pt-1">
              OVH 平台要求重新打开已关闭工单时必须提供详细原因说明，工程师将重新介入跟进。
            </DialogDescription>
          </DialogHeader>
          <div className="py-2">
            <Textarea
              value={reopenReason}
              onChange={(e) => setReopenReason(e.target.value)}
              placeholder="请陈述需要重新开启工单的理由与补充情况 (必填)..."
              rows={4}
              className="resize-none text-xs sm:text-sm bg-background/80"
            />
          </div>
          <DialogFooter className="gap-2 sm:gap-0 pt-2">
            <Button
              variant="outline"
              onClick={() => setReopenDialogOpen(false)}
              disabled={reopenMutation.isPending}
            >
              取消
            </Button>
            <Button
              onClick={handleConfirmReopen}
              disabled={!reopenReason.trim() || reopenMutation.isPending}
              className="bg-primary hover:bg-primary/90 text-primary-foreground"
            >
              {reopenMutation.isPending ? "提交中..." : "提交并重新打开"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
