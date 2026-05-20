import { useState, useEffect, useCallback } from "react";
import { api, type PendingReplyQueueRow, type PendingReplyKind } from "@/lib/api";

type ChannelFilter = "all" | "telegram" | "email";
type KindFilter = "all" | PendingReplyKind;

export function usePendingQueue() {
  const [rows, setRows] = useState<PendingReplyQueueRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [channelFilter, setChannelFilter] = useState<ChannelFilter>("all");
  const [kindFilter, setKindFilter] = useState<KindFilter>("all");
  const [lastUpdated, setLastUpdated] = useState<Date>(new Date());

  const fetchData = useCallback(async (isInitial: boolean) => {
    try {
      const data = await api.listPendingReplies();
      setRows(data);
      setLastUpdated(new Date());
    } catch {
      // Keep last-good state; the interval will retry.
    } finally {
      if (isInitial) setLoading(false);
    }
  }, []);

  useEffect(() => { fetchData(true); }, [fetchData]);
  useEffect(() => {
    const id = setInterval(() => fetchData(false), 10_000);
    return () => clearInterval(id);
  }, [fetchData]);

  // Optimistic remove on approve/reject so the queue feels instant.
  // On error refetch to recover; the dispatcher may have actually
  // succeeded even when the wire returned 5xx, so the source of truth
  // is the next list response, not local state.
  const handleApprove = async (id: string) => {
    try {
      await api.approvePendingReply(id);
      setRows((prev) => prev.filter((r) => r.id !== id));
    } catch {
      await fetchData(false);
    }
  };

  const handleReject = async (id: string) => {
    try {
      await api.rejectPendingReply(id);
      setRows((prev) => prev.filter((r) => r.id !== id));
    } catch {
      await fetchData(false);
    }
  };

  const handleEdited = (id: string, newBody: string) => {
    setRows((prev) => prev.map((r) => (r.id === id ? { ...r, body: newBody } : r)));
  };

  const filtered = rows.filter((r) => {
    if (channelFilter !== "all" && r.channel !== channelFilter) return false;
    if (kindFilter !== "all" && r.kind !== kindFilter) return false;
    return true;
  });

  return {
    rows,
    filtered,
    loading,
    lastUpdated,
    channelFilter,
    setChannelFilter,
    kindFilter,
    setKindFilter,
    handleApprove,
    handleReject,
    handleEdited,
  };
}
