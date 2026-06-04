import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import type { InboxFlowResponse } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  api: {
    getInboxAnalytics: vi.fn(),
  },
}));

import { api } from "@/lib/api";
import { POLL_INTERVAL_MS, useInboxAnalytics } from "./useInboxAnalytics";

function inbox(over: Partial<InboxFlowResponse> = {}): InboxFlowResponse {
  return {
    period: over.period ?? { from: "2026-05-01T00:00:00Z", to: "2026-06-01T00:00:00Z" },
    leads: over.leads ?? {
      total: 120,
      by_channel: { telegram: 70, email: 50 },
      by_status: { new: 10, qualified: 40, closed: 5 },
    },
    qualifications: over.qualifications ?? {
      score_histogram: [
        { range: "0-20", count: 5 },
        { range: "81-100", count: 25 },
      ],
      avg_score: 64.5,
    },
    pending_replies: over.pending_replies ?? {
      approved: 80,
      rejected: 10,
      currently_pending: 5,
      approve_rate: 0.842,
      p50_time_to_decide_seconds: 120,
      p95_time_to_decide_seconds: 600,
    },
  };
}

describe("useInboxAnalytics", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(api.getInboxAnalytics).mockResolvedValue(inbox());
  });

  it("fetches on mount with the default month period", async () => {
    const { result } = renderHook(() => useInboxAnalytics());
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(api.getInboxAnalytics).toHaveBeenCalledTimes(1);
    expect(api.getInboxAnalytics).toHaveBeenCalledWith("month");
    expect(result.current.data?.leads.total).toBe(120);
    expect(result.current.error).toBeNull();
    expect(result.current.lastUpdated).not.toBeNull();
  });

  it("refetches when the period changes", async () => {
    const { rerender } = renderHook(({ p }: { p: "week" | "month" | "all" }) => useInboxAnalytics(p), {
      initialProps: { p: "month" },
    });
    await waitFor(() => expect(api.getInboxAnalytics).toHaveBeenCalledTimes(1));

    rerender({ p: "week" });
    await waitFor(() => expect(api.getInboxAnalytics).toHaveBeenLastCalledWith("week"));
    await waitFor(() => expect(api.getInboxAnalytics).toHaveBeenCalledTimes(2));
  });

  it("polls every POLL_INTERVAL_MS", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      renderHook(() => useInboxAnalytics());
      await waitFor(() => expect(api.getInboxAnalytics).toHaveBeenCalledTimes(1));

      await act(async () => {
        await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
      });
      expect(api.getInboxAnalytics).toHaveBeenCalledTimes(2);
    } finally {
      vi.useRealTimers();
    }
  });

  it("surfaces fetch errors and preserves the last good data", async () => {
    vi.mocked(api.getInboxAnalytics).mockResolvedValueOnce(inbox());
    vi.mocked(api.getInboxAnalytics).mockRejectedValueOnce(new Error("network"));

    const { result } = renderHook(() => useInboxAnalytics());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.data).not.toBeNull();
    expect(result.current.error).toBeNull();

    await act(async () => {
      await result.current.refresh();
    });
    expect(result.current.error).not.toBeNull();
    // Last-good data preserved through the failed refresh (keep-last).
    expect(result.current.data?.leads.total).toBe(120);
  });
});
