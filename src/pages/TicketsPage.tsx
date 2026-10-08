import { useState, useEffect } from "react";
import { Helmet } from "react-helmet-async";
import { AppLayout } from "@/components/layout/AppLayout";
import { PageHeader } from "@/components/common/PageHeader";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useActiveServerControlAccount } from "@/hooks/use-active-account";
import { useAccounts } from "@/hooks/use-accounts";
import {
  type SupportTicket,
  useSupportTickets,
  useSupportTicket,
  type CreateTicketResponse,
} from "@/hooks/ovh/use-tickets";
import { TicketList } from "@/components/tickets/TicketList";
import { TicketChat } from "@/components/tickets/TicketChat";
import { CreateTicketDialog } from "@/components/tickets/CreateTicketDialog";
import { Ticket, Plus, MessageSquare, Headphones, Globe2, ShieldCheck, Sparkles } from "lucide-react";
import { EmptyState } from "@/components/common/EmptyState";
import { cn } from "@/lib/utils";

function TicketsPage() {
  const [activeAccount, setActiveAccount] = useActiveServerControlAccount();
  const { data: accounts } = useAccounts();

  // 筛选与搜索状态
  const [statusFilter, setStatusFilter] = useState("all");
  const [archivedFilter, setArchivedFilter] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedTicketId, setSelectedTicketId] = useState<number | null>(null);
  const [createDialogOpen, setCreateDialogOpen] = useState(false);

  // 首次加载或无账户时同步默认账户
  useEffect(() => {
    if (!accounts || accounts.length === 0) return;
    const exists = accounts.some((a) => a.id === activeAccount);
    if (!exists) {
      const def = accounts.find((a) => a.isDefault) || accounts[0];
      if (def?.id) {
        setActiveAccount(def.id);
      }
    }
  }, [accounts, activeAccount, setActiveAccount]);

  // 切换账户时清空选中的工单
  useEffect(() => {
    setSelectedTicketId(null);
  }, [activeAccount]);

  // 工单列表数据
  const {
    data: ticketData,
    isLoading: isTicketsLoading,
    isFetching: isTicketsFetching,
    refetch: refetchTickets,
  } = useSupportTickets({
    account: activeAccount,
    status: statusFilter,
    archived: archivedFilter,
    q: searchQuery,
    page: 1,
    pageSize: 50,
  });

  const tickets = ticketData?.tickets || [];

  // 单个工单实时详情
  const { data: currentTicketDetail } = useSupportTicket(selectedTicketId, activeAccount);

  // 桌面端自动选中第一个工单
  useEffect(() => {
    if (typeof window !== "undefined" && window.innerWidth >= 1024) {
      if (!selectedTicketId && tickets.length > 0) {
        setSelectedTicketId(tickets[0].ticketId);
      }
    }
  }, [tickets, selectedTicketId]);

  // 当前激活的完整工单数据
  const selectedTicket =
    currentTicketDetail ||
    tickets.find((t) => t.ticketId === selectedTicketId) ||
    null;

  const activeAcc = accounts?.find((a) => a.id === activeAccount);

  const handleTicketCreated = (res: CreateTicketResponse) => {
    setSelectedTicketId(res.ticketId);
    refetchTickets();
  };

  return (
    <div className="space-y-4">
      {/* ── 页面 Header ── */}
      <PageHeader
        icon={Ticket}
        title="支持工单中心"
        description={
          activeAcc
            ? `与 OVH 官方技术专家与客服实时协同 · 当前已连接账户：${activeAcc.name} (${activeAcc.zone})`
            : "与 OVH 官方技术专家与客服实时协同沟通"
        }
        action={
          <div className="flex items-center gap-2.5 flex-wrap">
            {/* 账户切换器 */}
            {accounts && accounts.length > 1 ? (
              <Select value={activeAccount} onValueChange={setActiveAccount}>
                <SelectTrigger className="h-9 w-[150px] sm:w-[190px] text-xs font-mono bg-card/80 border-border/80 rounded-xl shadow-xs">
                  <Globe2 className="h-3.5 w-3.5 mr-1.5 opacity-60 shrink-0" />
                  <SelectValue placeholder="切换账户" />
                </SelectTrigger>
                <SelectContent className="rounded-xl">
                  {accounts.map((a) => (
                    <SelectItem key={a.id} value={a.id} className="text-xs py-2">
                      <div className="font-medium">{a.name}</div>
                      <div className="text-[10px] text-muted-foreground font-mono">
                        区域: {a.zone} {a.isDefault ? "(默认)" : ""}
                      </div>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : activeAcc ? (
              <div className="text-xs font-mono text-muted-foreground px-3 py-1.5 rounded-xl bg-muted/60 border border-border/50 flex items-center gap-1.5">
                <Globe2 className="h-3.5 w-3.5 opacity-60" />
                <span>{activeAcc.name}</span>
                <span className="opacity-40">·</span>
                <span>{activeAcc.zone}</span>
              </div>
            ) : null}

            {/* 创建工单高光按钮 */}
            <Button
              size="sm"
              onClick={() => setCreateDialogOpen(true)}
              className="h-9 px-4 text-xs font-medium rounded-xl bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white shadow-[0_2px_12px_rgba(16,185,129,0.25)] hover:shadow-[0_4px_16px_rgba(16,185,129,0.35)] transition-all"
            >
              <Plus className="h-4 w-4 mr-1.5" />
              创建新工单
            </Button>
          </div>
        }
      />

      {/* ── 主体工作台：自适应双栏布局 ── */}
      <div className="h-[calc(100vh-12rem)] min-h-[600px] max-h-[960px] grid grid-cols-1 lg:grid-cols-12 gap-4">
        {/* 左侧列表栏 */}
        <div
          className={cn(
            "h-full min-h-0 lg:col-span-4 xl:col-span-4 transition-all duration-200",
            selectedTicketId ? "hidden lg:block" : "block"
          )}
        >
          <TicketList
            tickets={tickets}
            selectedTicketId={selectedTicketId}
            onSelectTicket={(t) => setSelectedTicketId(t.ticketId)}
            statusFilter={statusFilter}
            onStatusFilterChange={setStatusFilter}
            archivedFilter={archivedFilter}
            onArchivedFilterChange={setArchivedFilter}
            searchQuery={searchQuery}
            onSearchQueryChange={setSearchQuery}
            isLoading={isTicketsLoading}
            isFetching={isTicketsFetching}
            onRefresh={refetchTickets}
            warning={ticketData?.warning}
            incomplete={ticketData?.incomplete}
          />
        </div>

        {/* 右侧聊天窗口 */}
        <div
          className={cn(
            "h-full min-h-0 lg:col-span-8 xl:col-span-8 transition-all duration-200",
            !selectedTicketId ? "hidden lg:flex" : "flex flex-col"
          )}
        >
          {selectedTicket ? (
            <TicketChat
              ticket={selectedTicket}
              activeAccount={activeAccount}
              onBack={() => setSelectedTicketId(null)}
              onRefresh={refetchTickets}
            />
          ) : (
            <div className="h-full flex flex-col items-center justify-center p-8 bg-card/25 border border-border/60 rounded-2xl border-dashed backdrop-blur-md">
              <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-primary/10 border border-primary/20 text-primary mb-4 shadow-sm">
                <MessageSquare className="h-8 w-8" />
              </div>
              <h3 className="text-base font-semibold text-foreground tracking-tight">
                请选择工单开启对话
              </h3>
              <p className="text-xs sm:text-sm text-muted-foreground max-w-sm text-center mt-1.5 leading-relaxed">
                从左侧列表点击任意工单即可展开官方客服对话视窗，支持微信/即时通讯交互模式与实时回复。
              </p>
            </div>
          )}
        </div>
      </div>

      {/* ── 创建工单对话框 ── */}
      <CreateTicketDialog
        open={createDialogOpen}
        onOpenChange={setCreateDialogOpen}
        activeAccount={activeAccount}
        onCreated={handleTicketCreated}
      />
    </div>
  );
}

const Page = () => (
  <>
    <Helmet>
      <title>支持工单中心 | OVH WebUI</title>
    </Helmet>
    <AppLayout>
      <TicketsPage />
    </AppLayout>
  </>
);

export default Page;
