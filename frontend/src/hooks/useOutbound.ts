import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import { mapOutboundToUI, type UIMessage } from "@/components/outbound/constants";

type ChannelFilter = "all" | "email" | "telegram" | "phone_call";
type StatusFilter = "all" | "sent" | "approved" | "rejected";

export function useOutbound() {
  const [messages, setMessages] = useState<UIMessage[]>([]);
  const [sentMessages, setSentMessages] = useState<UIMessage[]>([]);
  const [tab, setTab] = useState<"queue" | "sent">("queue");
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState("");
  const [stats, setStats] = useState({ draft: 0, approved: 0, sent: 0, opened: 0, replied: 0, bounced: 0 });
  const [channelFilter, setChannelFilter] = useState<ChannelFilter>("all");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [page, setPage] = useState(1);
  const [lastUpdated, setLastUpdated] = useState<Date>(new Date());
  const [approvingAll, setApprovingAll] = useState(false);
  const [autopilot, setAutopilot] = useState(false);

  useEffect(() => { setPage(1); }, [tab, channelFilter, statusFilter, search]);

  const fetchData = useCallback(async (isInitial: boolean) => {
    try {
      const [queue, sent] = await Promise.all([api.getOutboundQueue(), api.getOutboundSent()]);
      setMessages(queue.map(mapOutboundToUI));
      setSentMessages(sent.map(mapOutboundToUI));
      setLastUpdated(new Date());
    } catch { /* ignore */ } finally { if (isInitial) setLoading(false); }
    api.getOutboundStats().then(setStats).catch(() => {});
  }, []);

  useEffect(() => { fetchData(true); }, [fetchData]);
  useEffect(() => { const id = setInterval(() => fetchData(false), 10_000); return () => clearInterval(id); }, [fetchData]);

  const refreshStats = () => { api.getOutboundStats().then(setStats).catch(() => {}); };

  const handleApprove = async (id: string) => {
    try { await api.approveMessage(id); setMessages((prev) => prev.filter((m) => m.id !== id)); refreshStats(); } catch { /* ignore */ }
  };

  const handleReject = async (id: string) => {
    try { await api.rejectMessage(id); setMessages((prev) => prev.filter((m) => m.id !== id)); refreshStats(); } catch { /* ignore */ }
  };

  const handleEdited = (id: string, newBody: string) => {
    setMessages((prev) => prev.map((m) => (m.id === id ? { ...m, body: newBody } : m)));
  };

  const handleApproveAll = async () => {
    setApprovingAll(true);
    try { for (const msg of messages) await api.approveMessage(msg.id); setMessages([]); refreshStats(); }
    catch { await fetchData(false); }
    finally { setApprovingAll(false); }
  };

  const activeList = tab === "queue" ? messages : sentMessages;
  const filtered = activeList.filter((m) => {
    if (search.trim()) {
      const q = search.toLowerCase();
      if (!m.name.toLowerCase().includes(q) && !m.body.toLowerCase().includes(q)) return false;
    }
    if (channelFilter !== "all" && m.channel !== channelFilter) return false;
    if (tab === "sent" && statusFilter !== "all" && m.status !== statusFilter) return false;
    return true;
  });

  const ITEMS_PER_PAGE = 10;
  const totalPages = Math.max(1, Math.ceil(filtered.length / ITEMS_PER_PAGE));
  const safePage = Math.min(page, totalPages);
  const paginatedItems = filtered.slice((safePage - 1) * ITEMS_PER_PAGE, safePage * ITEMS_PER_PAGE);

  return {
    messages, sentMessages, tab, setTab, loading, search, setSearch,
    stats, channelFilter, setChannelFilter, statusFilter, setStatusFilter,
    page, setPage, lastUpdated, approvingAll, autopilot, setAutopilot,
    filtered, paginatedItems, totalPages, safePage,
    handleApprove, handleReject, handleEdited, handleApproveAll,
    ITEMS_PER_PAGE,
  };
}
