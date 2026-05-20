"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  api,
  type AnalyticsPeriod,
  type CostRatiosResponse,
  type CostSummaryResponse,
} from "@/lib/api";

export const POLL_INTERVAL_MS = 30_000;

interface UseCostAnalyticsResult {
  ratios: CostRatiosResponse | null;
  summary: CostSummaryResponse | null;
  loading: boolean;
  error: Error | null;
  lastUpdated: Date | null;
  refresh: () => Promise<void>;
}

// daysForPeriod maps the period enum onto the day-span the
// cost-summary endpoint (which takes explicit from/to) needs. Mirrors
// the backend periodWindow logic — keeping the conversion client-side
// avoids a second backend round-trip just to derive dates.
function daysForPeriod(period: AnalyticsPeriod): number {
  if (period === "week") return 7;
  if (period === "month") return 30;
  // "all" — pick a very long span so the audit aggregation covers
  // everything the user could plausibly have logged.
  return 365 * 10;
}

// formatYMD turns a Date into the yyyy-mm-dd shape the cost-summary
// endpoint expects. Local-tz date is fine — the endpoint treats the
// strings as date boundaries, not instants.
function formatYMD(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

export function useCostAnalytics(period: AnalyticsPeriod = "month"): UseCostAnalyticsResult {
  const [ratios, setRatios] = useState<CostRatiosResponse | null>(null);
  const [summary, setSummary] = useState<CostSummaryResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const mountedRef = useRef(true);

  const fetchOnce = useCallback(async () => {
    try {
      const to = new Date();
      const from = new Date(to.getTime() - daysForPeriod(period) * 86_400_000);
      const [ratiosResp, summaryResp] = await Promise.all([
        api.getCostRatios(period),
        api.getCostSummary(formatYMD(from), formatYMD(to)),
      ]);
      if (!mountedRef.current) return;
      setRatios(ratiosResp);
      setSummary(summaryResp);
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

  return { ratios, summary, loading, error, lastUpdated, refresh: fetchOnce };
}
