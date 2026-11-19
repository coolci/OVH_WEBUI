import { useCallback, useEffect, useState } from "react";
import { Helmet } from "react-helmet-async";
import {
  ArrowLeft,
  Globe2,
  Headphones,
  MessageSquareText,
  Plus,
  RefreshCw,
} from "lucide-react";
import { AppLayout } from "@/components/layout/AppLayout";
import { PageHeader } from "@/components/common/PageHeader";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useActiveServerControlAccount } from "@/hooks/use-active-account";
import { useAccounts } from "@/hooks/use-accounts";
import {
  useSupportTickets,
  useSupportTicket,
  type CreateTicketResponse,
} from "@/hooks/ovh/use-tickets";
import { TicketList } from "@/components/tickets/TicketList";
import { TicketChat } from "@/components/tickets/TicketChat";
import { CreateTicketDialog } from "@/components/tickets/CreateTicketDialog";
import { cn } from "@/lib/utils";
import "@/components/tickets/tickets.css";

function TicketsPage() {
  const [activeAccount, setActiveAccount] = useActiveServerControlAccount();
  const { data: accounts } = useAccounts();
  const [statusFilter, setStatusFilter] = useState("all");
  const [archivedFilter, setArchivedFilter] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [selection, setSelection] = useState<{
    account: string;
    ticketId: number;
  } | null>(null);
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [isCompact, setIsCompact] = useState(
    () => window.matchMedia("(max-width: 1023px)").matches,
  );
  const selectedTicketId =
    selection?.account === activeAccount ? selection.ticketId : null;

  useEffect(() => {
    const media = window.matchMedia("(max-width: 1023px)");
    const update = () => setIsCompact(media.matches);
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);

  useEffect(() => {
    if (
      !accounts?.length ||
      accounts.some((account) => account.id === activeAccount)
    )
      return;
    setActiveAccount(
      (accounts.find((account) => account.isDefault) || accounts[0]).id,
    );
  }, [accounts, activeAccount, setActiveAccount]);

  const {
    data: ticketData,
    isLoading,
    isFetching,
    isError,
    refetch,
  } = useSupportTickets({
    account: activeAccount,
    status: statusFilter,
    archived: archivedFilter,
    q: searchQuery,
    page: 1,
    pageSize: 50,
  });
  const tickets = ticketData?.tickets;
  // 桌面端还没点选时直接用列表第一张，避免欢迎屏闪一下
  const activeTicketId =
    selectedTicketId ??
    (!isCompact && tickets && tickets.length > 0 ? tickets[0].ticketId : null);
  const draftKey = `${activeAccount}:${activeTicketId}`;
  const detail = useSupportTicket(activeTicketId, activeAccount);
  const selectedTicket =
    detail.data ||
    tickets?.find((ticket) => ticket.ticketId === activeTicketId);
  const activeAcc = accounts?.find((account) => account.id === activeAccount);
  const isResolvingTickets = isLoading || (isFetching && !tickets);
  const hasTickets = Boolean(tickets && tickets.length > 0);

  useEffect(() => {
    if (!isCompact && !selectedTicketId && tickets?.length) {
      setSelection({ account: activeAccount, ticketId: tickets[0].ticketId });
    }
  }, [isCompact, tickets, selectedTicketId, activeAccount]);

  const handleSearch = useCallback((query: string) => {
    setSearchQuery(query);
    setSelection(null);
  }, []);
  const handleTicketCreated = (response: CreateTicketResponse) => {
    setStatusFilter("all");
    setSearchQuery("");
    setSelection({ account: activeAccount, ticketId: response.ticketId });
    void refetch();
  };
  const updateDraft = (text: string) =>
    setDrafts((current) => ({ ...current, [draftKey]: text }));

  return (
    <div
      className={cn(
        "support-workspace",
        activeTicketId && "has-conversation",
      )}
    >
      <PageHeader
        className="support-page-header"
        icon={Headphones}
        title="支持工单"
        description="集中管理问题，让每一次沟通都有进展。"
        action={
          <div className="flex w-full items-center justify-between gap-2 sm:w-auto sm:justify-end">
            {accounts && accounts.length > 1 ? (
              <Select value={activeAccount} onValueChange={setActiveAccount}>
                <SelectTrigger
                  className="support-account-select"
                  aria-label="切换工单账户"
                >
                  <Globe2 className="mr-2 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                  <SelectValue placeholder="选择账户" />
                </SelectTrigger>
                <SelectContent>
                  {accounts.map((account) => (
                    <SelectItem
                      key={account.id}
                      value={account.id}
                      className="text-xs"
                    >
                      {account.name} · {account.zone}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : activeAcc ? (
              <div className="support-account-label">
                <Globe2 size={14} />
                <span>{activeAcc.name}</span>
                <span className="text-muted-foreground">{activeAcc.zone}</span>
              </div>
            ) : null}
            <Button
              size="sm"
              className="h-8 gap-1.5 rounded-lg px-3 text-xs font-medium shadow-sm"
              onClick={() => setCreateDialogOpen(true)}
            >
              <Plus className="h-3.5 w-3.5" />
              新建工单
            </Button>
          </div>
        }
      />

      <div className="support-inbox">
        <aside
          className={cn("support-queue", activeTicketId && "hide-on-compact")}
          aria-label="工单队列"
        >
          <TicketList
            tickets={tickets || []}
            selectedTicketId={activeTicketId}
            onSelectTicket={(ticket) =>
              setSelection({
                account: activeAccount,
                ticketId: ticket.ticketId,
              })
            }
            statusFilter={statusFilter}
            onStatusFilterChange={(value) => {
              setStatusFilter(value);
              setSelection(null);
            }}
            archivedFilter={archivedFilter}
            onArchivedFilterChange={(value) => {
              setArchivedFilter(value);
              setSelection(null);
            }}
            searchQuery={searchQuery}
            onSearchQueryChange={handleSearch}
            isLoading={isLoading}
            isFetching={isFetching}
            isError={isError}
            onRefresh={() => void refetch()}
            onCreate={() => setCreateDialogOpen(true)}
            warning={ticketData?.warning}
            incomplete={ticketData?.incomplete}
          />
        </aside>
        <section
          className={cn(
            "support-conversation-panel",
            !activeTicketId && "hide-on-compact",
          )}
          aria-label="工单沟通区"
        >
          {selectedTicket ? (
            <TicketChat
              key={`${activeAccount}:${selectedTicket.ticketId}`}
              ticket={selectedTicket}
              activeAccount={activeAccount}
              accountName={activeAcc?.name}
              accountZone={activeAcc?.zone}
              draft={drafts[draftKey] || ""}
              onDraftChange={updateDraft}
              onReplySent={(sentDraft) =>
                setDrafts((current) =>
                  current[draftKey] === sentDraft
                    ? { ...current, [draftKey]: "" }
                    : current,
                )
              }
              onBack={() => setSelection(null)}
              onRefresh={() => {
                void refetch();
                void detail.refetch();
              }}
            />
          ) : isResolvingTickets || (!isCompact && hasTickets && !selectedTicket) || activeTicketId ? (
            <div className="support-detail-loading">
              <button
                className="support-icon-button self-start lg:hidden"
                aria-label="返回工单列表"
                onClick={() => setSelection(null)}
              >
                <ArrowLeft size={18} />
              </button>
              <div className="support-empty-state">
                <RefreshCw
                  size={24}
                  className={cn(!detail.isError && "animate-spin text-primary")}
                />
                <h2>{detail.isError ? "暂时无法加载工单" : "正在加载工单…"}</h2>
                {detail.isError && (
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => void detail.refetch()}
                  >
                    重新加载
                  </Button>
                )}
              </div>
            </div>
          ) : (
            <div className="support-empty-state support-welcome">
              <span className="support-welcome-icon">
                <MessageSquareText size={22} strokeWidth={1.75} />
              </span>
              <span className="support-eyebrow">OVHCLOUD SUPPORT</span>
              <h2>从这里，开始解决问题</h2>
              <p>
                选择左侧工单查看沟通记录，
                <br />
                或发起新的咨询，让支持团队协助处理。
              </p>
              <Button
                variant="outline"
                size="sm"
                className="border-border/80 hover:bg-accent text-foreground"
                onClick={() => setCreateDialogOpen(true)}
              >
                <Plus className="mr-1.5 h-3.5 w-3.5" />
                新建工单
              </Button>
            </div>
          )}
        </section>
      </div>
      <CreateTicketDialog
        key={activeAccount}
        open={createDialogOpen}
        onOpenChange={setCreateDialogOpen}
        activeAccount={activeAccount}
        onCreated={handleTicketCreated}
      />
    </div>
  );
}

export default function Page() {
  return (
    <>
      <Helmet>
        <title>支持工单 | OVH WebUI</title>
      </Helmet>
      <AppLayout workspace>
        <TicketsPage />
      </AppLayout>
    </>
  );
}
