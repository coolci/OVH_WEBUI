import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import api from "@/lib/api";
import { useActiveAccount } from "@/hooks/use-active-account";
import type { SupportTicket, TicketMessage } from "@/lib/types";

export function useTickets(archived?: boolean, overrideAccountId?: string) {
  const { activeAccountId } = useActiveAccount();
  const accountId = overrideAccountId ?? activeAccountId ?? undefined;

  return useQuery({
    queryKey: ["support-tickets", archived, accountId],
    queryFn: async () => {
      const res = await api.getTickets(archived, accountId);
      return (res.tickets || []) as SupportTicket[];
    },
    staleTime: 30_000,
  });
}

export function useTicketDetail(ticketId: number | string | null) {
  const { activeAccountId } = useActiveAccount();

  return useQuery({
    queryKey: ["support-ticket", ticketId, activeAccountId],
    queryFn: async () => {
      if (!ticketId) return null;
      const res = await api.getTicketDetail(ticketId, activeAccountId || undefined);
      return (res.ticket || null) as SupportTicket | null;
    },
    enabled: !!ticketId,
  });
}

export function useTicketMessages(ticketId: number | string | null) {
  const { activeAccountId } = useActiveAccount();

  return useQuery({
    queryKey: ["support-ticket-messages", ticketId, activeAccountId],
    queryFn: async () => {
      if (!ticketId) return [];
      const res = await api.getTicketMessages(ticketId, activeAccountId || undefined);
      return (res.messages || []) as TicketMessage[];
    },
    enabled: !!ticketId,
    refetchInterval: 15_000,
  });
}

export function useReplyTicket() {
  const queryClient = useQueryClient();
  const { activeAccountId } = useActiveAccount();

  return useMutation({
    mutationFn: async ({
      ticketId,
      body,
      accountId,
    }: {
      ticketId: number | string;
      body: string;
      accountId?: string;
    }) => {
      return api.replyTicket(ticketId, body, accountId || activeAccountId || undefined);
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ["support-ticket-messages", variables.ticketId] });
      queryClient.invalidateQueries({ queryKey: ["support-tickets"] });
    },
  });
}

export function useCreateTicket() {
  const queryClient = useQueryClient();
  const { activeAccountId } = useActiveAccount();

  return useMutation({
    mutationFn: async ({
      ticket,
      accountId,
    }: {
      ticket: { category: string; subject: string; body: string; serviceName?: string; type?: string };
      accountId?: string;
    }) => {
      return api.createTicket(ticket, accountId || activeAccountId || undefined);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["support-tickets"] });
    },
  });
}
