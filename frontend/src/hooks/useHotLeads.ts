"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api, type HotLead, type HotLeadsParams } from "@/lib/api";

// POLL_INTERVAL_MS exported so tests advance timers by an exact multiple.
export const POLL_INTERVAL_MS = 30_000;

interface UseHotLeadsResult {
  leads: HotLead[];
  totalMatching: number;
  loading: boolean;
  error: Error | null;
  lastUpdated: Date | null;
  refresh: () => Promise<void>;
}

// useHotLeads returns the ranked hot-leads read model for the given
// filter. Polls every POLL_INTERVAL_MS so the operator dashboard stays
// fresh, and keeps the last good rows visible on a transient fetch
// error (the panel informs, not blanks-out). Refetches whenever any
// filter field changes.
export function useHotLeads(params: HotLeadsParams): UseHotLeadsResult {
  const [leads, setLeads] = useState<HotLead[]>([]);
  const [totalMatching, setTotalMatching] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);

  const mountedRef = useRef(true);

  // Destructure so the fetch callback depends on primitive values, not
  // a fresh object literal each render (which would re-fire the effect
  // every render and hammer the API).
  const { period, status, channel, limit } = params;

  const fetchOnce = useCallback(async () => {
    try {
      const data = await api.getHotLeads({ period, status, channel, limit });
      if (!mountedRef.current) return;
      setLeads(data.leads);
      setTotalMatching(data.total_matching);
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
  }, [period, status, channel, limit]);

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

  return { leads, totalMatching, loading, error, lastUpdated, refresh: fetchOnce };
}
