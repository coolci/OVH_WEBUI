import { useEffect, useMemo, useRef, useState } from "react";
import {
  ArrowDown,
  ArrowLeft,
  CheckCircle2,
  CircleAlert,
  CircleDot,
  Headphones,
  Info,
  MessageSquareText,
  MoreHorizontal,
  RefreshCw,
  RotateCcw,
  Search,
  Send,
  Smile,
  UserRound,
  X,
} from "lucide-react";
import {
  type SupportTicket,
  useSupportMessages,
  useReplyTicket,
  useCloseTicket,
  useReopenTicket,
} from "@/hooks/ovh/use-tickets";
import { formatDateTime } from "@/lib/format-os";
import { getStateMeta } from "./labels";
import { TicketDetails } from "./TicketDetails";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Skeleton } from "@/components/common/Skeleton";
import { cn } from "@/lib/utils";

interface TicketChatProps {
  ticket: SupportTicket;
  activeAccount?: string;
  accountName?: string;
  accountZone?: string;
  draft: string;
  onDraftChange: (text: string) => void;
  onReplySent: (sentDraft: string) => void;
  onBack?: () => void;
  onRefresh?: () => void;
}

const EMOJIS = ["😊", "👍", "🙏", "👌", "🤝", "🎉", "✅", "💻"];
const QUICK_REPLIES = [
  "感谢您的帮助！",
  "好的，我会按照您提供的步骤操作。",
  "问题仍然存在，请协助进一步排查。",
  "问题已经解决，感谢支持！",
];

function dayLabel(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleDateString("zh-CN", {
    year: "numeric",
    month: "long",
    day: "numeric",
  });
}

export function TicketChat({
  ticket,
  activeAccount,
  accountName,
  accountZone,
  draft,
  onDraftChange,
  onReplySent,
  onBack,
  onRefresh,
}: TicketChatProps) {
  const [closeDialogOpen, setCloseDialogOpen] = useState(false);
  const [reopenDialogOpen, setReopenDialogOpen] = useState(false);
  const [reopenReason, setReopenReason] = useState("");
  const [detailsOpen, setDetailsOpen] = useState(false);
  const [isWide, setIsWide] = useState(
    () => window.matchMedia("(min-width: 1440px)").matches,
  );
  const [emojiOpen, setEmojiOpen] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const [messageSearch, setMessageSearch] = useState("");
  const [showJump, setShowJump] = useState(false);
  const [hasNewMessages, setHasNewMessages] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const scrollRef = useRef<HTMLDivElement>(null);
  const stickToBottomRef = useRef(true);
  const lastCountRef = useRef(0);
  const { data, isLoading, isFetching, isError, refetch } = useSupportMessages(
    ticket.ticketId,
    activeAccount,
  );
  const messages = useMemo(
    () =>
      [...(data || [])].sort(
        (a, b) => Date.parse(a.creationDate) - Date.parse(b.creationDate),
      ),
    [data],
  );
  const replyMutation = useReplyTicket(ticket.ticketId, activeAccount);
  const closeMutation = useCloseTicket(ticket.ticketId, activeAccount);
  const reopenMutation = useReopenTicket(ticket.ticketId, activeAccount);
  const isOpen = ticket.state === "open";
  const isClosed = ticket.state === "closed";
  const state = getStateMeta(ticket.state);
  const filteredMessages = messageSearch.trim()
    ? messages.filter((message) =>
        message.body
          .toLocaleLowerCase()
          .includes(messageSearch.trim().toLocaleLowerCase()),
      )
    : messages;

  useEffect(() => {
    const media = window.matchMedia("(min-width: 1440px)");
    const update = () => setIsWide(media.matches);
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);

  useEffect(() => {
    const container = scrollRef.current;
    if (container && !messageSearch) {
      if (stickToBottomRef.current)
        container.scrollTo({ top: container.scrollHeight, behavior: "auto" });
      else if (messages.length > lastCountRef.current) setHasNewMessages(true);
    }
    lastCountRef.current = messages.length;
  }, [messages.length, messageSearch]);

  const insertText = (text: string) => {
    const input = textareaRef.current;
    const start = input?.selectionStart ?? draft.length;
    const end = input?.selectionEnd ?? draft.length;
    onDraftChange(draft.slice(0, start) + text + draft.slice(end));
    requestAnimationFrame(() => {
      input?.focus();
      input?.setSelectionRange(start + text.length, start + text.length);
    });
  };
  const sendReply = async () => {
    const sentDraft = draft;
    if (!sentDraft.trim() || !isOpen || replyMutation.isPending) return;
    try {
      await replyMutation.mutateAsync(sentDraft.trim());
      onReplySent(sentDraft);
      stickToBottomRef.current = true;
      textareaRef.current?.focus();
      scrollRef.current?.scrollTo({
        top: scrollRef.current.scrollHeight,
        behavior: "auto",
      });
    } catch {
      /* The mutation displays the error; keep the draft. */
    }
  };
  const confirmClose = async () => {
    if (!isOpen || !ticket.canBeClosed || closeMutation.isPending) return;
    try {
      await closeMutation.mutateAsync();
      setCloseDialogOpen(false);
      onRefresh?.();
    } catch {
      /* Handled by the mutation. */
    }
  };
  const confirmReopen = async () => {
    if (!isClosed || !reopenReason.trim() || reopenMutation.isPending) return;
    try {
      await reopenMutation.mutateAsync(reopenReason.trim());
      setReopenDialogOpen(false);
      setReopenReason("");
      onRefresh?.();
    } catch {
      /* Handled by the mutation. */
    }
  };
  const refresh = () => {
    void refetch();
    onRefresh?.();
  };

  return (
    <div className="ticket-conversation-workspace">
      <div className="ticket-conversation">
        <header className="ticket-conversation-header">
          <div className="ticket-conversation-topline">
            {onBack && (
              <button
                className="support-icon-button lg:hidden"
                aria-label="返回工单列表"
                onClick={onBack}
              >
                <ArrowLeft size={18} />
              </button>
            )}
            <span className="ticket-reference">
              工单 #{ticket.ticketNumber || ticket.ticketId}
            </span>
            <span className={cn("support-status", `is-${state.variant}`)}>
              {isClosed ? <CheckCircle2 size={12} /> : <CircleDot size={12} />}
              {state.label}
            </span>
            <div className="ml-auto flex items-center gap-1">
              <button
                className={cn(
                  "support-icon-button",
                  detailsOpen && "is-active",
                )}
                aria-label="工单详情"
                title="工单详情"
                aria-expanded={detailsOpen}
                onClick={() => setDetailsOpen(!detailsOpen)}
              >
                <Info size={17} />
              </button>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <button
                    className="support-icon-button"
                    aria-label="更多工单操作"
                    title="更多操作"
                  >
                    <MoreHorizontal size={19} />
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-44">
                  <DropdownMenuItem onSelect={() => setDetailsOpen(true)}>
                    <Info className="mr-2 h-4 w-4" />
                    查看工单详情
                  </DropdownMenuItem>
                  <DropdownMenuItem disabled={isFetching} onSelect={refresh}>
                    <RefreshCw className="mr-2 h-4 w-4" />
                    刷新沟通记录
                  </DropdownMenuItem>
                  {(isClosed || (isOpen && ticket.canBeClosed)) && (
                    <DropdownMenuSeparator />
                  )}
                  {isOpen && ticket.canBeClosed && (
                    <DropdownMenuItem onSelect={() => setCloseDialogOpen(true)}>
                      <CheckCircle2 className="mr-2 h-4 w-4" />
                      关闭工单
                    </DropdownMenuItem>
                  )}
                  {isClosed && (
                    <DropdownMenuItem
                      onSelect={() => setReopenDialogOpen(true)}
                    >
                      <RotateCcw className="mr-2 h-4 w-4" />
                      重新打开工单
                    </DropdownMenuItem>
                  )}
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>
          <h2 title={ticket.subject}>{ticket.subject}</h2>
          <div className="ticket-conversation-recipient">
            <span className="ticket-provider-mark">
              <Headphones size={11} />
            </span>
            OVHcloud 支持团队
            <span className="ticket-recipient-separator">/</span>
            <span>{accountName || "当前账户"}</span>
          </div>
        </header>

        <div className="ticket-thread-toolbar">
          <span>
            沟通记录{" "}
            <span className="ticket-message-count">{messages.length}</span>
          </span>
          <button
            className={cn("support-icon-button", searchOpen && "is-active")}
            aria-label="搜索聊天记录"
            title="搜索聊天记录"
            aria-expanded={searchOpen}
            onClick={() => {
              setSearchOpen(!searchOpen);
              if (searchOpen) setMessageSearch("");
            }}
          >
            <Search size={15} />
          </button>
        </div>
        {searchOpen && (
          <div className="ticket-message-search">
            <Search size={14} />
            <input
              autoFocus
              aria-label="搜索聊天记录"
              placeholder="搜索当前工单的沟通内容…"
              value={messageSearch}
              onChange={(event) => setMessageSearch(event.target.value)}
            />
            <span>
              {messageSearch.trim() ? `${filteredMessages.length} 条` : ""}
            </span>
            <button
              className="support-icon-button"
              aria-label="关闭聊天搜索"
              onClick={() => {
                setSearchOpen(false);
                setMessageSearch("");
              }}
            >
              <X size={14} />
            </button>
          </div>
        )}
        {isError && messages.length > 0 && (
          <div className="support-inline-warning">
            <CircleAlert size={14} />
            <span>暂时无法同步新消息，当前显示已加载的记录。</span>
            <button onClick={refresh}>重试</button>
          </div>
        )}

        <div className="ticket-thread-container">
          <div
            ref={scrollRef}
            className="ticket-thread support-scrollbar"
            aria-label="聊天记录"
            aria-busy={isLoading}
            onScroll={() => {
              const container = scrollRef.current;
              if (!container) return;
              const nearBottom =
                container.scrollHeight -
                  container.scrollTop -
                  container.clientHeight <
                100;
              stickToBottomRef.current = nearBottom;
              setShowJump(!nearBottom);
              if (nearBottom) setHasNewMessages(false);
            }}
          >
            {isLoading ? (
              <div className="space-y-6 p-2">
                <Skeleton className="h-28 w-5/6 rounded-xl" />
                <Skeleton className="ml-auto h-20 w-4/5 rounded-xl" />
                <Skeleton className="h-36 w-5/6 rounded-xl" />
              </div>
            ) : isError && messages.length === 0 ? (
              <div className="support-empty-state">
                <CircleAlert size={27} />
                <h3>沟通记录加载失败</h3>
                <p>请稍后重试，已输入的内容会保留。</p>
                <button className="support-text-button" onClick={refresh}>
                  重新加载消息
                </button>
              </div>
            ) : filteredMessages.length === 0 ? (
              <div className="support-empty-state">
                <MessageSquareText size={28} strokeWidth={1.5} />
                <h3>
                  {messageSearch.trim() ? "没有找到相关记录" : "暂无沟通记录"}
                </h3>
                <p>
                  {messageSearch.trim()
                    ? "尝试其他关键词。"
                    : "支持团队的回复会显示在这里。"}
                </p>
              </div>
            ) : (
              filteredMessages.map((message, index) => {
                const isMe = message.from === "customer";
                const previous = filteredMessages[index - 1];
                const showDate =
                  !previous ||
                  new Date(previous.creationDate).toDateString() !==
                    new Date(message.creationDate).toDateString();
                return (
                  <div key={message.messageId}>
                    {showDate && (
                      <div className="ticket-date-divider">
                        <span>{dayLabel(message.creationDate)}</span>
                      </div>
                    )}
                    <article
                      className={cn("ticket-message", isMe && "is-customer")}
                    >
                      <div className="ticket-message-heading">
                        <span
                          className={cn(
                            "ticket-sender-avatar",
                            isMe && "is-customer",
                          )}
                        >
                          {isMe ? (
                            <UserRound size={13} />
                          ) : (
                            <Headphones size={13} />
                          )}
                        </span>
                        <span className="ticket-sender-name">
                          {isMe ? "我" : "OVHcloud 支持团队"}
                        </span>
                        <time dateTime={message.creationDate}>
                          {formatDateTime(message.creationDate)}
                        </time>
                      </div>
                      <div className="ticket-message-card">{message.body}</div>
                    </article>
                  </div>
                );
              })
            )}
          </div>
          {showJump && !messageSearch && (
            <button
              className="ticket-jump-button"
              onClick={() => {
                stickToBottomRef.current = true;
                scrollRef.current?.scrollTo({
                  top: scrollRef.current.scrollHeight,
                  behavior: "auto",
                });
              }}
            >
              <ArrowDown size={13} />
              {hasNewMessages ? "查看新消息" : "回到最新"}
            </button>
          )}
        </div>

        {isOpen ? (
          <div className="ticket-composer-area">
            <div className="ticket-composer">
              <div className="ticket-composer-label">
                <MessageSquareText size={14} />
                <span>回复</span>
                <span className="ticket-composer-to">
                  发送至 OVHcloud 支持团队
                </span>
              </div>
              <textarea
                ref={textareaRef}
                className="ticket-reply support-scrollbar"
                aria-label="回复工单"
                placeholder="补充问题细节、排查结果，或回复支持团队…"
                value={draft}
                disabled={replyMutation.isPending}
                onChange={(event) => onDraftChange(event.target.value)}
                onKeyDown={(event) => {
                  if (event.nativeEvent.isComposing || event.keyCode === 229)
                    return;
                  if (
                    event.key === "Enter" &&
                    (event.ctrlKey || event.metaKey)
                  ) {
                    event.preventDefault();
                    void sendReply();
                  }
                }}
              />
              <div className="ticket-composer-actions">
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <button
                      className="ticket-template-button"
                      aria-label="常用回复"
                      disabled={replyMutation.isPending}
                    >
                      <MessageSquareText size={14} />
                      <span>常用回复</span>
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent
                    side="top"
                    align="start"
                    className="max-w-[calc(100vw-2rem)]"
                    onCloseAutoFocus={(event) => event.preventDefault()}
                  >
                    {QUICK_REPLIES.map((reply) => (
                      <DropdownMenuItem
                        key={reply}
                        className="text-xs sm:text-[13px] py-1.5 cursor-pointer"
                        onSelect={() => insertText(reply)}
                      >
                        {reply}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
                <Popover open={emojiOpen} onOpenChange={setEmojiOpen}>
                  <PopoverTrigger asChild>
                    <button
                      className="support-icon-button"
                      aria-label="表情"
                      title="表情"
                      disabled={replyMutation.isPending}
                    >
                      <Smile size={17} />
                    </button>
                  </PopoverTrigger>
                  <PopoverContent
                    side="top"
                    align="start"
                    className="w-64 p-3"
                    onCloseAutoFocus={(event) => event.preventDefault()}
                  >
                    <div className="grid grid-cols-8 gap-1">
                      {EMOJIS.map((emoji) => (
                        <button
                          key={emoji}
                          className="rounded p-1 text-xl hover:bg-muted"
                          aria-label={`插入表情 ${emoji}`}
                          onClick={() => {
                            insertText(emoji);
                            setEmojiOpen(false);
                          }}
                        >
                          {emoji}
                        </button>
                      ))}
                    </div>
                  </PopoverContent>
                </Popover>
                <Button
                  className="ticket-send-button ml-auto"
                  aria-label="发送回复"
                  disabled={!draft.trim() || replyMutation.isPending}
                  onClick={() => void sendReply()}
                >
                  {replyMutation.isPending ? (
                    <RefreshCw size={14} className="animate-spin" />
                  ) : (
                    <Send size={14} />
                  )}
                  {replyMutation.isPending ? "发送中" : "发送回复"}
                </Button>
              </div>
            </div>
            <div className="ticket-composer-help">
              <span>
                {draft ? "草稿已在当前页面保留" : "Enter 换行，支持多行内容"}
              </span>
              <span className="ticket-shortcut-hint">
                Ctrl / ⌘ + Enter 发送
              </span>
            </div>
          </div>
        ) : (
          <div className="ticket-closed-banner">
            <span className="ticket-closed-icon">
              {isClosed ? (
                <CheckCircle2 size={20} />
              ) : (
                <CircleAlert size={20} />
              )}
            </span>
            <div>
              <strong>
                {isClosed ? "此工单已关闭" : "当前工单暂不可回复"}
              </strong>
              <p>
                {isClosed
                  ? "仍有问题？重新打开工单，继续与支持团队沟通。"
                  : "刷新工单状态后再试。"}
              </p>
            </div>
            {isClosed && (
              <Button
                variant="outline"
                size="sm"
                className="h-8.5 px-3 text-xs font-medium"
                onClick={() => setReopenDialogOpen(true)}
              >
                <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
                重新打开
              </Button>
            )}
          </div>
        )}
      </div>

      {isWide && detailsOpen && (
        <aside className="ticket-details-panel" aria-label="工单详情面板">
          <div className="ticket-details-heading">
            <h3>工单详情</h3>
            <button
              className="support-icon-button"
              aria-label="关闭工单详情"
              onClick={() => setDetailsOpen(false)}
            >
              <X size={16} />
            </button>
          </div>
          <div className="support-scrollbar min-h-0 flex-1 overflow-y-auto">
            <TicketDetails
              ticket={ticket}
              accountName={accountName}
              accountZone={accountZone}
            />
          </div>
        </aside>
      )}
      <Sheet open={!isWide && detailsOpen} onOpenChange={setDetailsOpen}>
        <SheetContent className="w-[min(90vw,340px)] overflow-y-auto px-0">
          <SheetHeader className="px-5">
            <SheetTitle>工单详情</SheetTitle>
            <SheetDescription>状态、关联服务和时间记录</SheetDescription>
          </SheetHeader>
          <TicketDetails
            ticket={ticket}
            accountName={accountName}
            accountZone={accountZone}
          />
        </SheetContent>
      </Sheet>
      <Dialog open={closeDialogOpen} onOpenChange={setCloseDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="text-base font-semibold">
              关闭工单 #{ticket.ticketNumber || ticket.ticketId}？
            </DialogTitle>
            <DialogDescription className="text-xs sm:text-sm text-muted-foreground mt-1.5 leading-relaxed">
              支持团队会将此问题视为已解决。若仍需协助，之后可以随时重新打开此工单。
            </DialogDescription>
          </DialogHeader>
          <DialogFooter className="gap-2 sm:gap-2.5 pt-2">
            <Button
              variant="outline"
              className="h-9 sm:h-10 px-4 text-xs sm:text-sm font-medium"
              onClick={() => setCloseDialogOpen(false)}
              disabled={closeMutation.isPending}
            >
              继续沟通
            </Button>
            <Button
              variant="destructive"
              className="h-9 sm:h-10 px-4 text-xs sm:text-sm font-medium"
              onClick={() => void confirmClose()}
              disabled={closeMutation.isPending}
            >
              {closeMutation.isPending ? "关闭中…" : "确认关闭工单"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <Dialog open={reopenDialogOpen} onOpenChange={setReopenDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="text-base font-semibold">
              重新打开工单 #{ticket.ticketNumber || ticket.ticketId}
            </DialogTitle>
            <DialogDescription className="text-xs sm:text-sm text-muted-foreground mt-1.5 leading-relaxed">
              补充仍然存在的问题或重新打开的原因，支持团队将继续跟进。
            </DialogDescription>
          </DialogHeader>
          <Textarea
            aria-label="重新打开工单的原因"
            value={reopenReason}
            onChange={(event) => setReopenReason(event.target.value)}
            placeholder="描述仍然存在的问题或补充情况…"
            rows={4}
            className="resize-none text-xs sm:text-[13px] leading-relaxed rounded-xl p-3"
            disabled={reopenMutation.isPending}
          />
          <DialogFooter className="gap-2 sm:gap-2.5 pt-2">
            <Button
              variant="outline"
              className="h-9 sm:h-10 px-4 text-xs sm:text-sm font-medium"
              onClick={() => setReopenDialogOpen(false)}
              disabled={reopenMutation.isPending}
            >
              取消
            </Button>
            <Button
              className="h-9 sm:h-10 px-4 text-xs sm:text-sm font-medium bg-emerald-600 hover:bg-emerald-500 text-white"
              onClick={() => void confirmReopen()}
              disabled={!reopenReason.trim() || reopenMutation.isPending}
            >
              {reopenMutation.isPending ? "提交中…" : "重新打开"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
