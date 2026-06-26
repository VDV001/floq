import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import type { SequenceAnalyticsResponse } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  api: {
    getSequenceAnalytics: vi.fn(),
  },
}));

import { api } from "@/lib/api";
import { POLL_INTERVAL_MS, useSequenceAnalytics } from "./useSequenceAnalytics";

function response(over: Partial<SequenceAnalyticsResponse> = {}): SequenceAnalyticsResponse {
  return {
    sequences: over.sequences ?? [
      {
        id: "seq-1",
        name: "Cold Outreach IT",
        sent: 100,
        delivered: 95,
        opened: 45,
        replied: 12,
        converted: 4,
        open_rate: 0.473,
        reply_rate: 0.126,
        conversion_rate: 0.042,
      },
    ],
    period: over.period ?? "all",
  };
}

describe("useSequenceAnalytics", () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it("fetches on mount with default period and stops loading", async () => {
    vi.mocked(api.getSequenceAnalytics).mockResolvedValueOnce(response());

    const { result } = renderHook(() => useSequenceAnalytics());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });
    expect(api.getSequenceAnalytics).toHaveBeenCalledTimes(1);
    expect(api.getSequenceAnalytics).toHaveBeenCalledWith("all");
    expect(result.current.rows).toHaveLength(1);
    expect(result.current.error).toBeNull();
  });

  it("refetches when period changes", async () => {
    vi.mocked(api.getSequenceAnalytics).mockResolvedValue(response());

    const { result, rerender } = renderHook(
      ({ period }: { period: "week" | "month" | "all" }) => useSequenceAnalytics(period),
      { initialProps: { period: "all" } },
    );

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(api.getSequenceAnalytics).toHaveBeenLastCalledWith("all");

    rerender({ period: "week" });
    await waitFor(() => expect(api.getSequenceAnalytics).toHaveBeenLastCalledWith("week"));
  });

  it("polls every POLL_INTERVAL_MS while mounted", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      vi.mocked(api.getSequenceAnalytics).mockResolvedValue(response());
      renderHook(() => useSequenceAnalytics());

      await waitFor(() => expect(api.getSequenceAnalytics).toHaveBeenCalledTimes(1));

      await act(async () => {
        await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
      });
      expect(api.getSequenceAnalytics).toHaveBeenCalledTimes(2);
    } finally {
      vi.useRealTimers();
    }
  });

  it("surfaces error and keeps last good rows", async () => {
    vi.mocked(api.getSequenceAnalytics).mockResolvedValueOnce(response());
    vi.mocked(api.getSequenceAnalytics).mockRejectedValueOnce(new Error("network"));

    const { result } = renderHook(() => useSequenceAnalytics());

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.rows).toHaveLength(1);
    expect(result.current.error).toBeNull();

    await act(async () => {
      await result.current.refresh();
    });

    expect(result.current.error).not.toBeNull();
    expect(result.current.rows).toHaveLength(1); // last good rows preserved
  });

  it("wraps a non-Error rejection into an Error", async () => {
    vi.mocked(api.getSequenceAnalytics).mockResolvedValueOnce(response());
    const { result } = renderHook(() => useSequenceAnalytics());
    await waitFor(() => expect(result.current.loading).toBe(false));

    vi.mocked(api.getSequenceAnalytics).mockRejectedValueOnce("string failure");
    await act(async () => {
      await result.current.refresh();
    });
    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.error?.message).toBe("string failure");
  });

  it("refresh callback triggers a fetch", async () => {
    vi.mocked(api.getSequenceAnalytics).mockResolvedValue(response());

    const { result } = renderHook(() => useSequenceAnalytics("week"));

    await waitFor(() => expect(api.getSequenceAnalytics).toHaveBeenCalledTimes(1));

    await act(async () => {
      await result.current.refresh();
    });
    expect(api.getSequenceAnalytics).toHaveBeenCalledTimes(2);
  });
});
