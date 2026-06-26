"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api, type AnalyticsPeriod, type InboxFlowResponse } from "@/lib/api";

// POLL_INTERVAL_MS exported so tests advance timers by an exact multiple.
export const POLL_INTERVAL_MS = 30_000;

interface UseInboxAnalyticsResult {
  data: InboxFlowResponse | null;
  loading: boolean;
  error: Error | null;
  lastUpdated: Date | null;
  refresh: () => Promise<void>;
}

// useInboxAnalytics returns the View 3 inbound-flow read model for the
// given period. Polls every POLL_INTERVAL_MS so the dashboard stays
// fresh and keeps the last good data visible on a transient fetch error
// (the panel informs, not blanks-out). Mirrors useCostAnalytics.
export function useInboxAnalytics(period: AnalyticsPeriod = "month"): UseInboxAnalyticsResult {
  const [data, setData] = useState<InboxFlowResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const mountedRef = useRef(true);

  const fetchOnce = useCallback(async () => {
    try {
      const resp = await api.getInboxAnalytics(period);
      if (!mountedRef.current) return;
      setData(resp);
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

  return { data, loading, error, lastUpdated, refresh: fetchOnce };
}
