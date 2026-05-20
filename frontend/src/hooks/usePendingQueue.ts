import { useState, useEffect, useCallback } from "react";
import {
  api,
  type PendingReplyBulkDecision,
  type PendingReplyKind,
  type PendingReplyQueueRow,
} from "@/lib/api";

type ChannelFilter = "all" | "telegram" | "email";
type KindFilter = "all" | PendingReplyKind;

export const POLL_INTERVAL_MS = 10_000;

export function usePendingQueue() {
  const [rows, setRows] = useState<PendingReplyQueueRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [channelFilter, setChannelFilter] = useState<ChannelFilter>("all");
  const [kindFilter, setKindFilter] = useState<KindFilter>("all");
  const [lastUpdated, setLastUpdated] = useState<Date>(new Date());
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [bulkSummary, setBulkSummary] = useState<{ ok: number; failed: number } | null>(null);

  const fetchData = useCallback(async (isInitial: boolean) => {
    try {
      const data = await api.listPendingReplies();
      setRows(data);
      setLastUpdated(new Date());
    } catch (e) {
      // Keep last-good state; the interval will retry. Surface to the
      // console so a silent 5xx loop is debuggable from the browser.
      console.warn("pending queue refetch failed", e);
    } finally {
      if (isInitial) setLoading(false);
    }
  }, []);

  useEffect(() => { fetchData(true); }, [fetchData]);
  useEffect(() => {
    const id = setInterval(() => fetchData(false), POLL_INTERVAL_MS);
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

  const filtered = rows.filter((r) => {
    if (channelFilter !== "all" && r.channel !== channelFilter) return false;
    if (kindFilter !== "all" && r.kind !== kindFilter) return false;
    return true;
  });

  const toggleSelected = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const clearSelected = () => setSelectedIds(new Set());

  // Bulk apply the decision to every currently-selected row. Removes
  // rows that succeed from local state; rows that fail stay visible
  // so the operator can retry or read the error. Top-level error
  // (rare — shape problems are guarded client-side) triggers a
  // refetch so we converge with the server.
  const bulkDecide = async (decision: PendingReplyBulkDecision) => {
    const ids = Array.from(selectedIds);
    if (ids.length === 0) return;
    try {
      const { results } = await api.bulkPendingReplies({ ids, decision });
      const okIds = new Set(results.filter((r) => r.ok).map((r) => r.id));
      const ok = okIds.size;
      const failed = results.length - ok;
      setRows((prev) => prev.filter((r) => !okIds.has(r.id)));
      setSelectedIds(new Set());
      setBulkSummary({ ok, failed });
    } catch {
      await fetchData(false);
      setSelectedIds(new Set());
    }
  };

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
    // Selection + bulk
    selectedIds,
    toggleSelected,
    clearSelected,
    bulkApprove: () => bulkDecide("approve"),
    bulkReject: () => bulkDecide("reject"),
    bulkSummary,
    dismissBulkSummary: () => setBulkSummary(null),
  };
}
