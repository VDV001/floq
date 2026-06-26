"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  api,
  type AnalyticsPeriod,
  type QualificationDistributionResponse,
  type SequenceConversionResponse,
} from "@/lib/api";

export const POLL_INTERVAL_MS = 30_000;

interface UseFunnelAnalyticsResult {
  distribution: QualificationDistributionResponse | null;
  conversion: SequenceConversionResponse | null;
  loading: boolean;
  error: Error | null;
  lastUpdated: Date | null;
  refresh: () => Promise<void>;
}

// useFunnelAnalytics loads the matview-backed funnel read-models for the
// requested period (the matview is day-bucketed / per-entry, so the reader
// windows it server-side — see the analytics read-path ADR). Mirrors
// useSequenceAnalytics: fetch both in parallel, poll, expose a manual refresh.
export function useFunnelAnalytics(period: AnalyticsPeriod = "all"): UseFunnelAnalyticsResult {
  const [distribution, setDistribution] = useState<QualificationDistributionResponse | null>(null);
  const [conversion, setConversion] = useState<SequenceConversionResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const mountedRef = useRef(true);

  const fetchOnce = useCallback(async () => {
    try {
      const [dist, conv] = await Promise.all([
        api.getQualificationDistribution(period),
        api.getSequenceConversion(period),
      ]);
      if (!mountedRef.current) return;
      setDistribution(dist);
      setConversion(conv);
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

  return { distribution, conversion, loading, error, lastUpdated, refresh: fetchOnce };
}
