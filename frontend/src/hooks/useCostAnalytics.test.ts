import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import type { CostRatiosResponse, CostSummaryResponse } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  api: {
    getCostRatios: vi.fn(),
    getCostSummary: vi.fn(),
  },
}));

import { api } from "@/lib/api";
import { POLL_INTERVAL_MS, useCostAnalytics } from "./useCostAnalytics";

function ratios(over: Partial<CostRatiosResponse> = {}): CostRatiosResponse {
  return {
    period: over.period ?? { from: "2026-04-20T00:00:00Z", to: "2026-05-20T00:00:00Z" },
    total_cost_usd: over.total_cost_usd ?? 6.0,
    total_calls: over.total_calls ?? 42,
    leads_count: over.leads_count ?? 4,
    qualified_leads_count: over.qualified_leads_count ?? 2,
    converted_count: over.converted_count ?? 1,
    drafts_sent_count: over.drafts_sent_count ?? 5,
    cost_per_lead_usd: over.cost_per_lead_usd ?? 1.5,
    cost_per_qualified_lead_usd: over.cost_per_qualified_lead_usd ?? 3.0,
    cost_per_converted_usd: over.cost_per_converted_usd ?? 6.0,
    cost_per_draft_sent_usd: over.cost_per_draft_sent_usd ?? 1.2,
  };
}

function summary(over: Partial<CostSummaryResponse> = {}): CostSummaryResponse {
  return {
    total_usd: over.total_usd ?? 6.0,
    total_calls: over.total_calls ?? 42,
    by_request_type: over.by_request_type ?? [
      { request_type: "qualification", calls: 30, usd: 4.5, tokens_in: 1000, tokens_out: 500 },
      { request_type: "cold_message", calls: 12, usd: 1.5, tokens_in: 800, tokens_out: 400 },
    ],
    by_model: over.by_model ?? [
      { model: "claude-haiku-4-5", calls: 30, usd: 4.5, tokens_in: 1000, tokens_out: 500 },
    ],
    period: over.period ?? { from: "2026-04-20", to: "2026-05-20" },
  };
}

describe("useCostAnalytics", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(api.getCostRatios).mockResolvedValue(ratios());
    vi.mocked(api.getCostSummary).mockResolvedValue(summary());
  });

  it("fetches both endpoints on mount with default period", async () => {
    const { result } = renderHook(() => useCostAnalytics());

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(api.getCostRatios).toHaveBeenCalledTimes(1);
    expect(api.getCostRatios).toHaveBeenCalledWith("month");
    expect(api.getCostSummary).toHaveBeenCalledTimes(1);

    expect(result.current.ratios?.total_cost_usd).toBe(6.0);
    expect(result.current.summary?.by_request_type).toHaveLength(2);
    expect(result.current.error).toBeNull();
  });

  it("derives summary [from, to) ISO dates from the requested period", async () => {
    renderHook(() => useCostAnalytics("week"));

    await waitFor(() => expect(api.getCostSummary).toHaveBeenCalledTimes(1));

    const [from, to] = vi.mocked(api.getCostSummary).mock.calls[0]!;
    // Both ISO yyyy-mm-dd; span ≈ 7 days within tolerance.
    expect(from).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(to).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    const span = (new Date(to).getTime() - new Date(from).getTime()) / 86_400_000;
    expect(span).toBeGreaterThanOrEqual(6.5);
    expect(span).toBeLessThanOrEqual(7.5);
  });

  it("refetches both when period changes", async () => {
    const { rerender } = renderHook(({ p }: { p: "week" | "month" | "all" }) => useCostAnalytics(p), {
      initialProps: { p: "month" },
    });
    await waitFor(() => expect(api.getCostRatios).toHaveBeenCalledTimes(1));

    rerender({ p: "week" });
    await waitFor(() => expect(api.getCostRatios).toHaveBeenLastCalledWith("week"));
    await waitFor(() => expect(api.getCostSummary).toHaveBeenCalledTimes(2));
  });

  it("polls every POLL_INTERVAL_MS", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      renderHook(() => useCostAnalytics());
      await waitFor(() => expect(api.getCostRatios).toHaveBeenCalledTimes(1));

      await act(async () => {
        await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
      });
      expect(api.getCostRatios).toHaveBeenCalledTimes(2);
      expect(api.getCostSummary).toHaveBeenCalledTimes(2);
    } finally {
      vi.useRealTimers();
    }
  });

  it("surfaces error from either endpoint and preserves last good data", async () => {
    vi.mocked(api.getCostRatios).mockResolvedValueOnce(ratios());
    vi.mocked(api.getCostSummary).mockResolvedValueOnce(summary());
    vi.mocked(api.getCostRatios).mockRejectedValueOnce(new Error("network"));

    const { result } = renderHook(() => useCostAnalytics());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.ratios).not.toBeNull();
    expect(result.current.error).toBeNull();

    await act(async () => {
      await result.current.refresh();
    });
    expect(result.current.error).not.toBeNull();
    // Last-good ratios preserved through the failed refresh.
    expect(result.current.ratios?.total_cost_usd).toBe(6.0);
  });

  it("wraps a non-Error rejection into an Error", async () => {
    vi.mocked(api.getCostRatios).mockResolvedValueOnce(ratios());
    vi.mocked(api.getCostSummary).mockResolvedValueOnce(summary());
    const { result } = renderHook(() => useCostAnalytics());
    await waitFor(() => expect(result.current.loading).toBe(false));

    vi.mocked(api.getCostSummary).mockResolvedValueOnce(summary());
    vi.mocked(api.getCostRatios).mockRejectedValueOnce("string failure");
    await act(async () => {
      await result.current.refresh();
    });
    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.error?.message).toBe("string failure");
  });
});
