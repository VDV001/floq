"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api, type AnalyticsPeriod, type SequenceAnalyticsRow } from "@/lib/api";

// POLL_INTERVAL_MS is exported so tests can advance timers by an exact
// multiple without re-reading the constant from the hook file.
export const POLL_INTERVAL_MS = 30_000;

interface UseSequenceAnalyticsResult {
  rows: SequenceAnalyticsRow[];
  loading: boolean;
  error: Error | null;
  lastUpdated: Date | null;
  refresh: () => Promise<void>;
}

// useSequenceAnalytics returns the per-sequence performance read model
// for the requested period. Polls every POLL_INTERVAL_MS so the
// operator dashboard reflects newly opened/replied messages without a
// manual refresh. On error it keeps the last good rows visible — the
// metric panel is supposed to inform, not blank-out on transient
// network blips.
export function useSequenceAnalytics(period: AnalyticsPeriod = "all"): UseSequenceAnalyticsResult {
  const [rows, setRows] = useState<SequenceAnalyticsRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);

  // mountedRef guards against setState after unmount when the in-
  // flight fetch resolves late — happens routinely under fast-toggling
  // tabs and during test teardown.
  const mountedRef = useRef(true);

  const fetchOnce = useCallback(async () => {
    try {
      const data = await api.getSequenceAnalytics(period);
      if (!mountedRef.current) return;
      setRows(data.sequences);
      setError(null);
      setLastUpdated(new Date());
    } catch (err) {
      if (!mountedRef.current) return;
      setError(err instanceof Error ? err : new Error(String(err)));
    } finally {
      if (mountedRef.current) {
        setLoading(false);
      }
    }
  }, [period]);

  useEffect(() => {
    mountedRef.current = true;
    setLoading(true);
    void fetchOnce();
    const interval = setInterval(() => {
      void fetchOnce();
    }, POLL_INTERVAL_MS);
    return () => {
      mountedRef.current = false;
      clearInterval(interval);
    };
  }, [fetchOnce]);

  return { rows, loading, error, lastUpdated, refresh: fetchOnce };
}
