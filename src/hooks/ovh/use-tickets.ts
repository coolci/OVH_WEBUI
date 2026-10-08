import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/http";
import { apiMessage } from "@/lib/api-error";
import { toast } from "sonner";

export interface SupportTicket {
  ticketId: number;
  ticketNumber: number;
  subject: string;
  state: "open" | "closed" | "unknown" | string;
  category?: string;
  product?: string;
  serviceName?: string;
  lastMessageFrom: string;
  canBeClosed: boolean;
  creationDate: string;
  updateDate: string;
  type?: string;
  score?: string;
}

export interface SupportMessage {
  messageId: number;
  ticketId: number;
  body: string;
  from: string; // "customer" 或者是 OVH 支持客服
  creationDate: string;
  updateDate: string;
}

export interface SupportTicketsResponse {
  success: boolean;
  tickets: SupportTicket[];
  incomplete?: boolean;
  warning?: string;
}

export interface SupportTicketDetailResponse {
  success: boolean;
  ticket: SupportTicket;
}

export interface SupportTicketMessagesResponse {
  success: boolean;
  messages: SupportMessage[];
}

export interface CreateTicketParams {
  subject: string;
  body: string;
  category: string;
  subcategory: string;
  product: string;
  serviceName?: string;
}

export interface CreateTicketResponse {
  success: boolean;
  ticketId: number;
  ticketNumber: number;
  messageId: number;
  additionalNotice?: string;
}

export interface ListTicketsFilters {
  account?: string;
  status?: string; // "open" | "closed" | "all"
  archived?: boolean;
  q?: string;
  page?: number;
  pageSize?: number;
}

/** 查询工单列表 */
export function useSupportTickets(filters: ListTicketsFilters = {}) {
  const { account, status = "all", archived = false, q = "", page = 1, pageSize = 30 } = filters;

  return useQuery<SupportTicketsResponse>({
    queryKey: ["support-tickets", account, status, archived, q, page, pageSize],
    queryFn: async () => {
      const params: Record<string, unknown> = {
        page,
        pageSize,
        archived,
      };
      if (status && status !== "all") {
        params.status = status;
      }
      if (q && q.trim()) {
        params.q = q.trim();
      }
      if (account) {
        params.account = account;
      }

      const res = await api.get<SupportTicketsResponse>("/support/tickets", { params });
      return res.data;
    },
    refetchInterval: 30_000, // 30秒静默刷新一次列表
  });
}

/** 查询单张工单详情 */
export function useSupportTicket(ticketId?: number | null, account?: string) {
  return useQuery<SupportTicket | null>({
    queryKey: ["support-ticket", ticketId, account],
    queryFn: async () => {
      if (!ticketId) return null;
      const res = await api.get<SupportTicketDetailResponse>(`/support/tickets/${ticketId}`, {
        params: account ? { account } : undefined,
      });
      return res.data.ticket;
    },
    enabled: Boolean(ticketId),
    refetchInterval: 20_000,
  });
}

/** 查询单张工单的消息记录 */
export function useSupportMessages(ticketId?: number | null, account?: string) {
  return useQuery<SupportMessage[]>({
    queryKey: ["support-messages", ticketId, account],
    queryFn: async () => {
      if (!ticketId) return [];
      const res = await api.get<SupportTicketMessagesResponse>(`/support/tickets/${ticketId}/messages`, {
        params: account ? { account } : undefined,
      });
      return res.data.messages || [];
    },
    enabled: Boolean(ticketId),
    refetchInterval: 15_000, // 聊天界面 15 秒轮询一次新回复
  });
}

/** 创建工单 Mutation */
export function useCreateTicket(account?: string) {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: async (input: CreateTicketParams) => {
      const res = await api.post<CreateTicketResponse>("/support/tickets", input, {
        params: account ? { account } : undefined,
      });
      return res.data;
    },
    onSuccess: (data) => {
      toast.success(`工单 #${data.ticketNumber || data.ticketId} 创建成功！`, {
        description: data.additionalNotice || "OVH 官方技术支持将在收到后进行处理",
      });
      qc.invalidateQueries({ queryKey: ["support-tickets"] });
    },
    onError: (err) => {
      toast.error("创建工单失败", {
        description: apiMessage(err),
      });
    },
  });
}

/** 回复工单 Mutation */
export function useReplyTicket(ticketId: number, account?: string) {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: async (body: string) => {
      const res = await api.post<{ success: boolean }>(
        `/support/tickets/${ticketId}/reply`,
        { body },
        { params: account ? { account } : undefined }
      );
      return res.data;
    },
    onSuccess: () => {
      toast.success("回复已发送给 OVH 支持团队");
      qc.invalidateQueries({ queryKey: ["support-messages", ticketId] });
      qc.invalidateQueries({ queryKey: ["support-ticket", ticketId] });
      qc.invalidateQueries({ queryKey: ["support-tickets"] });
    },
    onError: (err) => {
      toast.error("发送回复失败", {
        description: apiMessage(err),
      });
    },
  });
}

/** 关闭工单 Mutation */
export function useCloseTicket(ticketId: number, account?: string) {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: async () => {
      const res = await api.post<{ success: boolean }>(
        `/support/tickets/${ticketId}/close`,
        {},
        { params: account ? { account } : undefined }
      );
      return res.data;
    },
    onSuccess: () => {
      toast.success(`工单 #${ticketId} 已成功关闭`);
      qc.invalidateQueries({ queryKey: ["support-ticket", ticketId] });
      qc.invalidateQueries({ queryKey: ["support-tickets"] });
    },
    onError: (err) => {
      toast.error("关闭工单失败", {
        description: apiMessage(err),
      });
    },
  });
}

/** 重新打开工单 Mutation */
export function useReopenTicket(ticketId: number, account?: string) {
  const qc = useQueryClient();

  return useMutation({
    mutationFn: async (body: string) => {
      const res = await api.post<{ success: boolean }>(
        `/support/tickets/${ticketId}/reopen`,
        { body },
        { params: account ? { account } : undefined }
      );
      return res.data;
    },
    onSuccess: () => {
      toast.success(`工单 #${ticketId} 已重新激活`);
      qc.invalidateQueries({ queryKey: ["support-messages", ticketId] });
      qc.invalidateQueries({ queryKey: ["support-ticket", ticketId] });
      qc.invalidateQueries({ queryKey: ["support-tickets"] });
    },
    onError: (err) => {
      toast.error("重新激活工单失败", {
        description: apiMessage(err),
      });
    },
  });
}
