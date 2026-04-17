import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import type { OutboundMessage, OutboundStats, UserSettings } from "@/lib/api";

// --- Mock api module ---
const mockApi = {
  getOutboundQueue: vi.fn(),
  getOutboundSent: vi.fn(),
  getOutboundStats: vi.fn(),
  approveMessage: vi.fn(),
  rejectMessage: vi.fn(),
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
};

vi.mock("@/lib/api", () => ({
  api: mockApi,
}));

// --- Factories ---
function makeOutboundMessage(overrides: Partial<OutboundMessage> = {}): OutboundMessage {
  return {
    id: "msg-1",
    prospect_id: "prospect-abc123",
    sequence_id: "seq-00000001",
    step_order: 1,
    channel: "email",
    body: "Hello prospect",
    status: "draft",
    scheduled_at: "2026-04-17T10:00:00Z",
    sent_at: null,
    created_at: "2026-04-17T09:00:00Z",
    ...overrides,
  };
}

function makeStats(overrides: Partial<OutboundStats> = {}): OutboundStats {
  return { draft: 5, approved: 3, sent: 10, opened: 2, replied: 1, bounced: 0, ...overrides };
}

function defaultSettings(): Partial<UserSettings> {
  return {
    auto_qualify: true,
    auto_draft: true,
    auto_send: false,
    auto_send_delay_min: 5,
    auto_followup: true,
    auto_followup_days: 2,
    auto_prospect_to_lead: true,
    auto_verify_import: false,
  };
}

// =====================
// useOutbound
// =====================
describe("useOutbound", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mockApi.getOutboundQueue.mockResolvedValue([]);
    mockApi.getOutboundSent.mockResolvedValue([]);
    mockApi.getOutboundStats.mockResolvedValue(makeStats());
    mockApi.approveMessage.mockResolvedValue(undefined);
    mockApi.rejectMessage.mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  async function renderOutbound() {
    const { useOutbound } = await import("./useOutbound");
    return renderHook(() => useOutbound());
  }

  it("loads queue and sent on mount via Promise.all", async () => {
    const queue = [makeOutboundMessage({ id: "q1" }), makeOutboundMessage({ id: "q2" })];
    const sent = [makeOutboundMessage({ id: "s1", status: "sent" })];
    mockApi.getOutboundQueue.mockResolvedValue(queue);
    mockApi.getOutboundSent.mockResolvedValue(sent);

    const { result } = await renderOutbound();

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.messages.length).toBe(2);
    expect(result.current.sentMessages.length).toBe(1);
    expect(mockApi.getOutboundQueue).toHaveBeenCalled();
    expect(mockApi.getOutboundSent).toHaveBeenCalled();
  });

  it("sets loading=false after initial fetch", async () => {
    const { result } = await renderOutbound();

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });
  });

  it("fetches stats on mount", async () => {
    const stats = makeStats({ draft: 99 });
    mockApi.getOutboundStats.mockResolvedValue(stats);

    const { result } = await renderOutbound();

    await waitFor(() => {
      expect(result.current.stats.draft).toBe(99);
    });
  });

  it("handleApprove removes message from queue and refreshes stats", async () => {
    const queue = [makeOutboundMessage({ id: "a1" }), makeOutboundMessage({ id: "a2" })];
    mockApi.getOutboundQueue.mockResolvedValue(queue);
    mockApi.approveMessage.mockResolvedValue(undefined);
    mockApi.getOutboundStats.mockResolvedValue(makeStats({ approved: 10 }));

    const { result } = await renderOutbound();

    await waitFor(() => {
      expect(result.current.messages.length).toBe(2);
    });

    await act(async () => {
      await result.current.handleApprove("a1");
    });

    expect(result.current.messages.length).toBe(1);
    expect(result.current.messages[0].id).toBe("a2");
    expect(mockApi.approveMessage).toHaveBeenCalledWith("a1");
  });

  it("handleReject removes message from queue", async () => {
    const queue = [makeOutboundMessage({ id: "r1" }), makeOutboundMessage({ id: "r2" })];
    mockApi.getOutboundQueue.mockResolvedValue(queue);
    mockApi.rejectMessage.mockResolvedValue(undefined);

    const { result } = await renderOutbound();

    await waitFor(() => {
      expect(result.current.messages.length).toBe(2);
    });

    await act(async () => {
      await result.current.handleReject("r1");
    });

    expect(result.current.messages.length).toBe(1);
    expect(result.current.messages[0].id).toBe("r2");
    expect(mockApi.rejectMessage).toHaveBeenCalledWith("r1");
  });

  it("handleEdited updates message body in-place", async () => {
    const queue = [makeOutboundMessage({ id: "e1", body: "Old body" })];
    mockApi.getOutboundQueue.mockResolvedValue(queue);

    const { result } = await renderOutbound();

    await waitFor(() => {
      expect(result.current.messages.length).toBe(1);
    });

    act(() => {
      result.current.handleEdited("e1", "New body");
    });

    expect(result.current.messages[0].body).toBe("New body");
  });

  it("handleApproveAll approves all messages and clears queue", async () => {
    const queue = [
      makeOutboundMessage({ id: "all1" }),
      makeOutboundMessage({ id: "all2" }),
      makeOutboundMessage({ id: "all3" }),
    ];
    mockApi.getOutboundQueue.mockResolvedValue(queue);
    mockApi.approveMessage.mockResolvedValue(undefined);

    const { result } = await renderOutbound();

    await waitFor(() => {
      expect(result.current.messages.length).toBe(3);
    });

    // Clear mock to ignore any calls from initial fetch / polling
    mockApi.approveMessage.mockClear();

    await act(async () => {
      await result.current.handleApproveAll();
    });

    expect(mockApi.approveMessage).toHaveBeenCalledTimes(3);
    expect(mockApi.approveMessage).toHaveBeenCalledWith("all1");
    expect(mockApi.approveMessage).toHaveBeenCalledWith("all2");
    expect(mockApi.approveMessage).toHaveBeenCalledWith("all3");
    expect(result.current.messages.length).toBe(0);
    expect(result.current.approvingAll).toBe(false);
  });

  it("handleApproveAll falls back to fetchData on error", async () => {
    const queue = [makeOutboundMessage({ id: "fail1" })];
    mockApi.getOutboundQueue.mockResolvedValue(queue);
    mockApi.approveMessage.mockRejectedValueOnce(new Error("network error"));

    const { result } = await renderOutbound();

    await waitFor(() => {
      expect(result.current.messages.length).toBe(1);
    });

    // Reset call count before approveAll triggers refetch
    mockApi.getOutboundQueue.mockClear();
    mockApi.getOutboundSent.mockClear();

    await act(async () => {
      await result.current.handleApproveAll();
    });

    // Should have called fetchData(false) as fallback
    expect(mockApi.getOutboundQueue).toHaveBeenCalled();
    expect(result.current.approvingAll).toBe(false);
  });

  describe("filtering", () => {
    beforeEach(() => {
      const queue = [
        makeOutboundMessage({ id: "f1", channel: "email", body: "Email about partnership" }),
        makeOutboundMessage({ id: "f2", channel: "telegram", body: "Telegram outreach" }),
        makeOutboundMessage({ id: "f3", channel: "email", body: "Another email" }),
      ];
      mockApi.getOutboundQueue.mockResolvedValue(queue);
    });

    it("filters by search query (name or body)", async () => {
      const { result } = await renderOutbound();

      await waitFor(() => {
        expect(result.current.messages.length).toBe(3);
      });

      act(() => {
        result.current.setSearch("partnership");
      });

      expect(result.current.filtered.length).toBe(1);
      expect(result.current.filtered[0].id).toBe("f1");
    });

    it("filters by channel", async () => {
      const { result } = await renderOutbound();

      await waitFor(() => {
        expect(result.current.messages.length).toBe(3);
      });

      act(() => {
        result.current.setChannelFilter("telegram");
      });

      expect(result.current.filtered.length).toBe(1);
      expect(result.current.filtered[0].id).toBe("f2");
    });

    it("channel 'all' shows everything", async () => {
      const { result } = await renderOutbound();

      await waitFor(() => {
        expect(result.current.messages.length).toBe(3);
      });

      act(() => {
        result.current.setChannelFilter("all");
      });

      expect(result.current.filtered.length).toBe(3);
    });

    it("status filter only applies in sent tab", async () => {
      const sent = [
        makeOutboundMessage({ id: "s1", status: "sent" }),
        makeOutboundMessage({ id: "s2", status: "approved" }),
      ];
      mockApi.getOutboundSent.mockResolvedValue(sent);

      const { result } = await renderOutbound();

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      // In queue tab, statusFilter should not apply
      act(() => {
        result.current.setStatusFilter("sent");
      });

      expect(result.current.filtered.length).toBe(3); // queue tab ignores status filter

      // Switch to sent tab
      act(() => {
        result.current.setTab("sent");
      });

      expect(result.current.filtered.length).toBe(1);
      expect(result.current.filtered[0].id).toBe("s1");
    });

    it("search is case-insensitive", async () => {
      const { result } = await renderOutbound();

      await waitFor(() => {
        expect(result.current.messages.length).toBe(3);
      });

      act(() => {
        result.current.setSearch("PARTNERSHIP");
      });

      expect(result.current.filtered.length).toBe(1);
    });

    it("empty/whitespace search shows all messages", async () => {
      const { result } = await renderOutbound();

      await waitFor(() => {
        expect(result.current.messages.length).toBe(3);
      });

      act(() => {
        result.current.setSearch("   ");
      });

      // trim() makes it empty, so no search filtering
      expect(result.current.filtered.length).toBe(3);
    });
  });

  describe("pagination", () => {
    it("paginates with ITEMS_PER_PAGE=10", async () => {
      const queue = Array.from({ length: 25 }, (_, i) =>
        makeOutboundMessage({ id: `p${i}` })
      );
      mockApi.getOutboundQueue.mockResolvedValue(queue);

      const { result } = await renderOutbound();

      await waitFor(() => {
        expect(result.current.messages.length).toBe(25);
      });

      expect(result.current.ITEMS_PER_PAGE).toBe(10);
      expect(result.current.totalPages).toBe(3);
      expect(result.current.paginatedItems.length).toBe(10);
      expect(result.current.safePage).toBe(1);
    });

    it("safePage clamps to totalPages when page exceeds it", async () => {
      const queue = Array.from({ length: 5 }, (_, i) =>
        makeOutboundMessage({ id: `c${i}` })
      );
      mockApi.getOutboundQueue.mockResolvedValue(queue);

      const { result } = await renderOutbound();

      await waitFor(() => {
        expect(result.current.messages.length).toBe(5);
      });

      act(() => {
        result.current.setPage(999);
      });

      // totalPages = 1, safePage should clamp to 1
      expect(result.current.safePage).toBe(1);
      expect(result.current.paginatedItems.length).toBe(5);
    });

    it("page 2 shows next batch", async () => {
      const queue = Array.from({ length: 15 }, (_, i) =>
        makeOutboundMessage({ id: `pg${i}` })
      );
      mockApi.getOutboundQueue.mockResolvedValue(queue);

      const { result } = await renderOutbound();

      await waitFor(() => {
        expect(result.current.messages.length).toBe(15);
      });

      act(() => {
        result.current.setPage(2);
      });

      expect(result.current.paginatedItems.length).toBe(5);
      expect(result.current.paginatedItems[0].id).toBe("pg10");
    });

    it("totalPages is at least 1 even with 0 items", async () => {
      mockApi.getOutboundQueue.mockResolvedValue([]);

      const { result } = await renderOutbound();

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      expect(result.current.totalPages).toBe(1);
    });
  });

  describe("page reset on filter/tab/search change", () => {
    it("resets page to 1 when tab changes", async () => {
      const queue = Array.from({ length: 15 }, (_, i) =>
        makeOutboundMessage({ id: `t${i}` })
      );
      mockApi.getOutboundQueue.mockResolvedValue(queue);

      const { result } = await renderOutbound();

      await waitFor(() => {
        expect(result.current.messages.length).toBe(15);
      });

      act(() => {
        result.current.setPage(2);
      });
      expect(result.current.page).toBe(2);

      act(() => {
        result.current.setTab("sent");
      });
      expect(result.current.page).toBe(1);
    });

    it("resets page to 1 when channelFilter changes", async () => {
      const { result } = await renderOutbound();

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      act(() => {
        result.current.setPage(3);
      });

      act(() => {
        result.current.setChannelFilter("telegram");
      });

      expect(result.current.page).toBe(1);
    });

    it("resets page to 1 when statusFilter changes", async () => {
      const { result } = await renderOutbound();

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      act(() => {
        result.current.setPage(3);
      });

      act(() => {
        result.current.setStatusFilter("sent");
      });

      expect(result.current.page).toBe(1);
    });

    it("resets page to 1 when search changes", async () => {
      const { result } = await renderOutbound();

      await waitFor(() => {
        expect(result.current.loading).toBe(false);
      });

      act(() => {
        result.current.setPage(3);
      });

      act(() => {
        result.current.setSearch("test");
      });

      expect(result.current.page).toBe(1);
    });
  });

  it("polls every 10s", async () => {
    const { result } = await renderOutbound();

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    const callsBefore = mockApi.getOutboundQueue.mock.calls.length;

    await act(async () => {
      vi.advanceTimersByTime(10_000);
    });

    expect(mockApi.getOutboundQueue.mock.calls.length).toBeGreaterThan(callsBefore);
  });
});

// =====================
// useAutomations
// =====================
describe("useAutomations", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mockApi.getSettings.mockResolvedValue(defaultSettings());
    mockApi.updateSettings.mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  async function renderAutomations() {
    const { useAutomations } = await import("./useAutomations");
    return renderHook(() => useAutomations());
  }

  it("loads toggles from settings via TOGGLE_MAP on mount", async () => {
    mockApi.getSettings.mockResolvedValue({
      ...defaultSettings(),
      auto_qualify: false,
      auto_send: true,
    });

    const { result } = await renderAutomations();

    await waitFor(() => {
      expect(result.current.toggles["auto-qualify"]).toBe(false);
    });

    expect(result.current.toggles["auto-send"]).toBe(true);
    expect(result.current.toggles["auto-draft"]).toBe(true);
  });

  it("loads input values from settings on mount", async () => {
    mockApi.getSettings.mockResolvedValue({
      ...defaultSettings(),
      auto_send_delay_min: 15,
      auto_followup_days: 7,
    });

    const { result } = await renderAutomations();

    await waitFor(() => {
      expect(result.current.inputs["auto-send"]).toBe(15);
    });

    expect(result.current.inputs["auto-followup"]).toBe(7);
  });

  it("uses default values when settings fields are falsy", async () => {
    mockApi.getSettings.mockResolvedValue({
      ...defaultSettings(),
      auto_send_delay_min: 0,
      auto_followup_days: 0,
    });

    const { result } = await renderAutomations();

    await waitFor(() => {
      // `s.auto_send_delay_min || 5` — 0 is falsy, so defaults to 5
      expect(result.current.inputs["auto-send"]).toBe(5);
    });

    expect(result.current.inputs["auto-followup"]).toBe(2);
  });

  it("toggle flips one automation and triggers debounced save", async () => {
    const { result } = await renderAutomations();

    await waitFor(() => {
      expect(result.current.toggles["auto-qualify"]).toBe(true);
    });

    act(() => {
      result.current.toggle("auto-qualify");
    });

    // Should flip from true to false
    expect(result.current.toggles["auto-qualify"]).toBe(false);

    // Save should not have been called yet (debounced 500ms)
    expect(mockApi.updateSettings).not.toHaveBeenCalled();

    // Advance past debounce
    await act(async () => {
      vi.advanceTimersByTime(500);
    });

    expect(mockApi.updateSettings).toHaveBeenCalledOnce();
    const savedData = mockApi.updateSettings.mock.calls[0][0];
    expect(savedData.auto_qualify).toBe(false);
  });

  it("toggleAll sets all to false when all are on", async () => {
    mockApi.getSettings.mockResolvedValue({
      ...defaultSettings(),
      auto_qualify: true,
      auto_draft: true,
      auto_send: true,
      auto_followup: true,
      auto_prospect_to_lead: true,
      auto_verify_import: true,
    });

    const { result } = await renderAutomations();

    await waitFor(() => {
      expect(result.current.toggles["auto-send"]).toBe(true);
    });

    act(() => {
      result.current.toggleAll();
    });

    // All should be false now
    expect(Object.values(result.current.toggles).every((v) => v === false)).toBe(true);
  });

  it("toggleAll sets all to true when some are off", async () => {
    const { result } = await renderAutomations();

    await waitFor(() => {
      expect(result.current.toggles["auto-qualify"]).toBe(true);
    });

    // Default has auto_send=false, auto_verify_import=false, so not all on
    act(() => {
      result.current.toggleAll();
    });

    // Since not all were on, should set all to true
    expect(Object.values(result.current.toggles).every((v) => v === true)).toBe(true);
  });

  it("updateInput changes value and triggers debounced save", async () => {
    const { result } = await renderAutomations();

    await waitFor(() => {
      expect(result.current.toggles["auto-qualify"]).toBe(true);
    });

    // Clear any calls from auto-advancing timers during waitFor
    mockApi.updateSettings.mockClear();

    act(() => {
      result.current.updateInput("auto-send", 30);
    });

    expect(result.current.inputs["auto-send"]).toBe(30);

    await act(async () => {
      vi.advanceTimersByTime(500);
    });

    expect(mockApi.updateSettings).toHaveBeenCalled();
    // Find the call that contains our updated value
    const calls = mockApi.updateSettings.mock.calls;
    const lastCall = calls[calls.length - 1][0];
    expect(lastCall.auto_send_delay_min).toBe(30);
  });

  it("debounce cancels previous timer on rapid calls", async () => {
    const { result } = await renderAutomations();

    await waitFor(() => {
      expect(result.current.toggles["auto-qualify"]).toBe(true);
    });

    // Clear any calls from auto-advancing timers during waitFor
    mockApi.updateSettings.mockClear();

    act(() => {
      result.current.toggle("auto-qualify");
    });

    // Immediately toggle again (within 500ms) — should cancel first timer
    act(() => {
      result.current.toggle("auto-draft");
    });

    // Advance past debounce for the second call
    await act(async () => {
      vi.advanceTimersByTime(500);
    });

    // Only one save should have happened (the second debounce replaces the first)
    expect(mockApi.updateSettings).toHaveBeenCalledOnce();
  });

  it("handles settings fetch failure gracefully", async () => {
    mockApi.getSettings.mockRejectedValue(new Error("network error"));

    const { result } = await renderAutomations();

    // Wait a tick for the rejected promise to settle
    await act(async () => {
      await Promise.resolve();
    });

    // Should still have default toggles (from AUTOMATIONS defaultOn)
    expect(result.current.toggles["auto-qualify"]).toBe(true);
    expect(result.current.toggles["auto-send"]).toBe(false);
  });
});
