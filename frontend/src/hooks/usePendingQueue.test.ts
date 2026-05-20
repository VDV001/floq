import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import type { PendingReplyQueueRow } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  api: {
    listPendingReplies: vi.fn(),
    approvePendingReply: vi.fn(),
    rejectPendingReply: vi.fn(),
  },
}));

import { api } from "@/lib/api";
import { POLL_INTERVAL_MS, usePendingQueue } from "./usePendingQueue";

function row(over: Partial<PendingReplyQueueRow> = {}): PendingReplyQueueRow {
  return {
    id: over.id ?? "pr-1",
    lead_id: over.lead_id ?? "lead-1",
    channel: over.channel ?? "telegram",
    kind: over.kind ?? "booking_link",
    body: over.body ?? "draft",
    status: "pending",
    created_at: over.created_at ?? "2026-05-20T10:00:00Z",
    lead: over.lead ?? {
      contact_name: "X",
      company: "Y",
      channel: "telegram",
    },
  };
}

describe("usePendingQueue", () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it("fetches once on mount and stops loading", async () => {
    vi.mocked(api.listPendingReplies).mockResolvedValueOnce([row({ id: "a" })]);

    const { result } = renderHook(() => usePendingQueue());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });
    expect(api.listPendingReplies).toHaveBeenCalledTimes(1);
    expect(result.current.rows).toHaveLength(1);
    expect(result.current.lastUpdated).toBeInstanceOf(Date);
  });

  it("polls every POLL_INTERVAL_MS", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      vi.mocked(api.listPendingReplies).mockResolvedValue([]);
      renderHook(() => usePendingQueue());

      // Initial fetch — microtasks flush via shouldAdvanceTime.
      await waitFor(() => {
        expect(api.listPendingReplies).toHaveBeenCalledTimes(1);
      });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
      });
      expect(api.listPendingReplies).toHaveBeenCalledTimes(2);

      await act(async () => {
        await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS * 2);
      });
      expect(api.listPendingReplies).toHaveBeenCalledTimes(4);
    } finally {
      vi.useRealTimers();
    }
  });

  it("keeps last-good state when polling errors out", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    try {
      vi.mocked(api.listPendingReplies)
        .mockResolvedValueOnce([row({ id: "alpha" })])
        .mockRejectedValueOnce(new Error("5xx"));
      const warn = vi.spyOn(console, "warn").mockImplementation(() => {});

      const { result } = renderHook(() => usePendingQueue());

      await waitFor(() => {
        expect(result.current.rows).toHaveLength(1);
      });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
      });

      // Last-good row stays — operator never sees an empty screen on a
      // transient backend hiccup. console.warn fires for ops visibility.
      expect(result.current.rows).toHaveLength(1);
      expect(result.current.rows[0]!.id).toBe("alpha");
      expect(warn).toHaveBeenCalled();
      warn.mockRestore();
    } finally {
      vi.useRealTimers();
    }
  });

  it("refetches when approve fails (recovery path)", async () => {
    vi.mocked(api.listPendingReplies)
      .mockResolvedValueOnce([row({ id: "a" })])
      .mockResolvedValueOnce([row({ id: "a" }), row({ id: "b" })]);
    vi.mocked(api.approvePendingReply).mockRejectedValueOnce(new Error("boom"));

    const { result } = renderHook(() => usePendingQueue());

    await waitFor(() => {
      expect(result.current.rows).toHaveLength(1);
    });

    await act(async () => {
      await result.current.handleApprove("a");
    });

    expect(api.approvePendingReply).toHaveBeenCalledWith("a");
    expect(api.listPendingReplies).toHaveBeenCalledTimes(2);
    expect(result.current.rows).toHaveLength(2);
  });

  it("kind filter narrows visible rows", async () => {
    vi.mocked(api.listPendingReplies).mockResolvedValueOnce([
      row({ id: "a", kind: "booking_link" }),
    ]);

    const { result } = renderHook(() => usePendingQueue());

    await waitFor(() => {
      expect(result.current.filtered).toHaveLength(1);
    });

    act(() => {
      result.current.setKindFilter("booking_link");
    });
    expect(result.current.filtered).toHaveLength(1);

    // No "other" kind exists yet on the type side; cast through unknown
    // to exercise the negative branch. When a second kind ships, swap
    // for a real value.
    act(() => {
      (result.current.setKindFilter as (k: string) => void)("__no_match__");
    });
    expect(result.current.filtered).toHaveLength(0);
  });

  it("channel filter excludes rows from the other channel", async () => {
    vi.mocked(api.listPendingReplies).mockResolvedValueOnce([
      row({ id: "tg", channel: "telegram" }),
      row({
        id: "em",
        channel: "email",
        lead: { contact_name: "E", company: "F", channel: "email" },
      }),
    ]);

    const { result } = renderHook(() => usePendingQueue());

    await waitFor(() => {
      expect(result.current.rows).toHaveLength(2);
    });

    act(() => {
      result.current.setChannelFilter("telegram");
    });
    expect(result.current.filtered.map((r) => r.id)).toEqual(["tg"]);

    act(() => {
      result.current.setChannelFilter("email");
    });
    expect(result.current.filtered.map((r) => r.id)).toEqual(["em"]);
  });
});
