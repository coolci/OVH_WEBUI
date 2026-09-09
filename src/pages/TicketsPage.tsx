import { AppLayout } from "@/components/layout/AppLayout";
import { Helmet } from "react-helmet-async";
import {
  MessageSquare, RefreshCw, Plus, Send, Clock, User, HelpCircle,
  CheckCircle2, AlertCircle, Inbox, Server
} from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { PageHeader } from "@/components/common/PageHeader";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Chip } from "@/components/common/Chip";
import { Skeleton } from "@/components/common/Skeleton";
import { EmptyState } from "@/components/common/EmptyState";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  useTickets,
  useTicketMessages,
  useReplyTicket,
  useCreateTicket,
} from "@/hooks/use-tickets";
import { useOwnedServers } from "@/hooks/use-server-control";
import type { SupportTicket } from "@/lib/types";

function ticketStateChip(state: string): { label: string; tone: "info" | "success" | "warning" | "danger" | "default" } {
  const s = (state || "").toLowerCase();
  if (s === "open") return { label: "处理中", tone: "info" };
  if (s === "answered") return { label: "官方已回复", tone: "success" };
  if (s === "closed") return { label: "已结单", tone: "default" };
  return { label: state || "未知", tone: "default" };
}

function TicketsPage() {
  const { data: tickets, isLoading, isFetching, refetch } = useTickets();
  const [selectedTicket, setSelectedTicket] = useState<SupportTicket | null>(null);
  const [openCreate, setOpenCreate] = useState(false);
  const [filterState, setFilterState] = useState("all");
  const [search, setSearch] = useState("");

  const ticketList = tickets || [];
  const filtered = ticketList.filter((t) => {
    if (filterState !== "all" && t.state?.toLowerCase() !== filterState.toLowerCase()) return false;
    if (search.trim()) {
      const q = search.trim().toLowerCase();
      if (!t.subject?.toLowerCase().includes(q) && !String(t.ticketId).includes(q)) return false;
    }
    return true;
  });

  const activeSelected = selectedTicket || filtered[0] || null;

  return (
    <div className="space-y-6">
      <PageHeader
        icon={MessageSquare}
        title="官方工单支持中心"
        description="查看 OVHcloud 官方技术支持工单、在线追踪进展并快速追问客服"
        action={
          <div className="flex gap-2">
            <Button variant="outline" onClick={() => refetch()} disabled={isFetching}>
              <RefreshCw className={`w-4 h-4 mr-1 ${isFetching ? "animate-spin" : ""}`} />
              刷新
            </Button>
            <Button onClick={() => setOpenCreate(true)}>
              <Plus className="w-4 h-4 mr-1" />
              新建工单
            </Button>
          </div>
        }
      />

      <div className="grid grid-cols-1 lg:grid-cols-[380px_1fr] gap-4 items-start">
        {/* 左侧：工单列表与过滤 */}
        <Card className="h-[calc(100vh-14rem)] flex flex-col overflow-hidden">
          <div className="p-3 border-b border-border space-y-2">
            <Input
              placeholder="搜索工单主题或编号..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="h-8 text-xs"
            />
            <div className="flex gap-1.5 overflow-x-auto pb-1">
              {[
                { id: "all", label: "全部" },
                { id: "open", label: "处理中" },
                { id: "answered", label: "已回复" },
                { id: "closed", label: "已结单" },
              ].map((tab) => (
                <button
                  key={tab.id}
                  onClick={() => setFilterState(tab.id)}
                  className={`px-2.5 py-1 rounded-md text-[11px] font-medium whitespace-nowrap transition-colors ${
                    filterState === tab.id
                      ? "bg-primary text-primary-foreground"
                      : "bg-muted text-muted-foreground hover:text-foreground"
                  }`}
                >
                  {tab.label}
                </button>
              ))}
            </div>
          </div>

          <div className="flex-1 overflow-y-auto divide-y divide-border">
            {isLoading ? (
              <div className="p-4 space-y-3">
                {Array.from({ length: 4 }).map((_, i) => (
                  <Skeleton key={i} className="h-16 rounded-lg" />
                ))}
              </div>
            ) : filtered.length === 0 ? (
              <div className="p-8 text-center text-xs text-muted-foreground">
                <Inbox className="w-8 h-8 mx-auto mb-2 text-muted-foreground/60" />
                没有找到工单记录
              </div>
            ) : (
              filtered.map((t) => {
                const isSelected = activeSelected?.ticketId === t.ticketId;
                const stateInfo = ticketStateChip(t.state);
                return (
                  <div
                    key={t.ticketId}
                    onClick={() => setSelectedTicket(t)}
                    className={`p-3.5 cursor-pointer transition-colors text-left space-y-1.5 ${
                      isSelected
                        ? "bg-primary/10 border-l-2 border-l-primary"
                        : "hover:bg-muted/50"
                    }`}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className="font-mono text-xs text-muted-foreground">
                        #{t.ticketNumber || t.ticketId}
                      </span>
                      <Chip tone={stateInfo.tone} className="text-[10px] px-1.5 py-0">
                        {stateInfo.label}
                      </Chip>
                    </div>
                    <div className="font-medium text-xs line-clamp-2 text-foreground">
                      {t.subject}
                    </div>
                    <div className="flex items-center justify-between text-[10px] text-muted-foreground pt-1">
                      <span className="uppercase">{t.category}</span>
                      <span>{new Date(t.updateDate || t.creationDate).toLocaleDateString("zh-CN")}</span>
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </Card>

        {/* 右侧：工单消息对话流 */}
        <Card className="h-[calc(100vh-14rem)] flex flex-col overflow-hidden">
          {activeSelected ? (
            <TicketDetailPane ticket={activeSelected} />
          ) : (
            <div className="flex-1 flex items-center justify-center p-8 text-center text-muted-foreground text-xs">
              请在左侧选择要查看的工单
            </div>
          )}
        </Card>
      </div>

      {openCreate && <CreateTicketDialog onClose={() => setOpenCreate(false)} />}
    </div>
  );
}

// ────────────────────────────── 工单详情与对话流 ──────────────────────────────

function TicketDetailPane({ ticket }: { ticket: SupportTicket }) {
  const { data: messages, isLoading, refetch } = useTicketMessages(ticket.ticketId);
  const reply = useReplyTicket();
  const [replyText, setReplyText] = useState("");

  const stateInfo = ticketStateChip(ticket.state);

  const handleSendReply = async () => {
    if (!replyText.trim()) return;
    try {
      await reply.mutateAsync({
        ticketId: ticket.ticketId,
        body: replyText.trim(),
      });
      toast.success("回复已提交给客服");
      setReplyText("");
      refetch();
    } catch (e: any) {
      toast.error(e?.response?.data?.error || "回复提交失败");
    }
  };

  return (
    <div className="flex flex-col h-full min-h-0">
      {/* 头部摘要 */}
      <div className="p-4 border-b border-border bg-muted/20 space-y-2">
        <div className="flex items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <span className="font-mono text-sm font-semibold text-primary">
              #{ticket.ticketNumber || ticket.ticketId}
            </span>
            <Chip tone={stateInfo.tone}>{stateInfo.label}</Chip>
            <Chip tone="default" className="text-[10px] uppercase">
              {ticket.category}
            </Chip>
          </div>
          <div className="text-xs text-muted-foreground">
            创建于: {new Date(ticket.creationDate).toLocaleString("zh-CN")}
          </div>
        </div>
        <h3 className="font-semibold text-sm leading-snug">{ticket.subject}</h3>
        {ticket.serviceName && (
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground font-mono">
            <Server className="w-3.5 h-3.5" />
            关联资产: {ticket.serviceName}
          </div>
        )}
      </div>

      {/* 消息对话列表 */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {isLoading ? (
          <div className="space-y-3">
            <Skeleton className="h-20 rounded-xl" />
            <Skeleton className="h-24 rounded-xl" />
          </div>
        ) : !messages || messages.length === 0 ? (
          <div className="text-center p-8 text-xs text-muted-foreground">
            暂无历史对话记录
          </div>
        ) : (
          messages.map((m) => {
            const isCustomer = m.from?.toLowerCase() === "customer";
            return (
              <div
                key={m.messageId}
                className={`flex flex-col gap-1 max-w-[85%] ${
                  isCustomer ? "ml-auto items-end" : "mr-auto items-start"
                }`}
              >
                <div className="flex items-center gap-2 text-[11px] text-muted-foreground px-1">
                  <span className="font-medium">
                    {isCustomer ? "我 (用户)" : "OVH 官方技术支持"}
                  </span>
                  <span>{new Date(m.creationDate).toLocaleString("zh-CN")}</span>
                </div>
                <div
                  className={`p-3.5 rounded-2xl text-xs whitespace-pre-wrap leading-relaxed shadow-sm ${
                    isCustomer
                      ? "bg-primary text-primary-foreground rounded-tr-sm"
                      : "bg-muted border border-border rounded-tl-sm"
                  }`}
                >
                  {m.body}
                </div>
              </div>
            );
          })
        )}
      </div>

      {/* 底部追问回复框 */}
      <div className="p-3 border-t border-border bg-card">
        <div className="flex gap-2">
          <Textarea
            rows={2}
            value={replyText}
            onChange={(e) => setReplyText(e.target.value)}
            placeholder="输入追加回复内容..."
            className="text-xs resize-none min-h-[56px]"
          />
          <Button
            className="h-auto px-4"
            disabled={!replyText.trim() || reply.isPending}
            onClick={handleSendReply}
          >
            <Send className="w-4 h-4 mr-1" />
            {reply.isPending ? "发送中…" : "发送"}
          </Button>
        </div>
      </div>
    </div>
  );
}

// ────────────────────────────── 新建工单弹窗 ──────────────────────────────

function CreateTicketDialog({ onClose }: { onClose: () => void }) {
  const create = useCreateTicket();
  const { data: servers } = useOwnedServers();

  const [category, setCategory] = useState("technical");
  const [subject, setSubject] = useState("");
  const [body, setBody] = useState("");
  const [serviceName, setServiceName] = useState("");

  const canSubmit = subject.trim() && body.trim() && category;

  const handleSubmit = async () => {
    if (!canSubmit) return;
    try {
      await create.mutateAsync({
        ticket: {
          category,
          subject: subject.trim(),
          body: body.trim(),
          serviceName: serviceName || undefined,
        },
      });
      toast.success("支持工单已成功提交至 OVH");
      onClose();
    } catch (e: any) {
      toast.error(e?.response?.data?.error || "创建工单失败");
    }
  };

  return (
    <Dialog open onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>新建官方支持工单</DialogTitle>
          <DialogDescription>
            直接向 OVH 官方工程师提交技术排障、网络咨询或硬件巡检申请。
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-3.5 py-2 text-xs">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="text-muted-foreground block mb-1 font-medium">工单分类 *</label>
              <Select value={category} onValueChange={setCategory}>
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="technical">技术与硬件排障 (Technical)</SelectItem>
                  <SelectItem value="billing">账单与支付 (Billing)</SelectItem>
                  <SelectItem value="assistance">售前与常规咨询 (Assistance)</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div>
              <label className="text-muted-foreground block mb-1 font-medium">关联服务器 (可选)</label>
              <Select value={serviceName || "none"} onValueChange={(v) => setServiceName(v === "none" ? "" : v)}>
                <SelectTrigger className="h-8 text-xs">
                  <SelectValue placeholder="不关联特定服务器" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">不关联特定服务器</SelectItem>
                  {(servers || [])
                    .filter((s) => Boolean(s.serviceName))
                    .map((s) => (
                      <SelectItem key={s.serviceName} value={s.serviceName}>
                        {s.name || s.serviceName}
                      </SelectItem>
                    ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          <div>
            <label className="text-muted-foreground block mb-1 font-medium">工单主题 *</label>
            <Input
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
              placeholder="例如: 独服硬件报错巡检申请 / 网络路由异常排查"
              className="h-8 text-xs"
              autoFocus
            />
          </div>

          <div>
            <label className="text-muted-foreground block mb-1 font-medium">问题详细描述 *</label>
            <Textarea
              rows={5}
              value={body}
              onChange={(e) => setBody(e.target.value)}
              placeholder="请尽可能详细描述遇到的问题、报错日志、复现步骤或涉及的 IP 地址..."
              className="text-xs"
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            取消
          </Button>
          <Button onClick={handleSubmit} disabled={!canSubmit || create.isPending}>
            {create.isPending ? "提交中…" : "提交工单"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

const Page = () => (
  <>
    <Helmet>
      <title>工单支持中心 | OVH WebUI</title>
    </Helmet>
    <AppLayout>
      <TicketsPage />
    </AppLayout>
  </>
);

export default Page;
