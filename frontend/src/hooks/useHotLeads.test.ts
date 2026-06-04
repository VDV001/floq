import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import type { HotLeadsResponse } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  api: {
    getHotLeads: vi.fn(),
  },
}));

import { api } from "@/lib/api";
import { useHotLeads } from "./useHotLeads";

function response(over: Partial<HotLeadsResponse> = {}): HotLeadsResponse {
  return {
    leads: over.leads ?? [
      {
        id: "lead-1",
        contact_name: "Acme Corp",
        channel: "telegram",
        status: "qualified",
        score: 87,
        score_reason: "strong fit",
        last_activity_at: "2026-05-19T14:23:00Z",
        qualified_at: "2026-05-19T14:23:00Z",
      },
    ],
    total_matching: over.total_matching ?? 45,
    limit_applied: over.limit_applied ?? 20,
  };
}

describe("useHotLeads", () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it("fetches on mount with the given filter and stops loading", async () => {
    vi.mocked(api.getHotLeads).mockResolvedValueOnce(response());

    const { result } = renderHook(() =>
      useHotLeads({ period: "month", status: "qualified", channel: "telegram" }),
    );

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(api.getHotLeads).toHaveBeenCalledWith({ period: "month", status: "qualified", channel: "telegram" });
    expect(result.current.leads).toHaveLength(1);
    expect(result.current.totalMatching).toBe(45);
    expect(result.current.error).toBeNull();
  });

  it("refetches when the filter changes", async () => {
    vi.mocked(api.getHotLeads).mockResolvedValue(response());

    const { result, rerender } = renderHook(
      ({ status }: { status: "any" | "qualified" }) => useHotLeads({ status }),
      { initialProps: { status: "any" as "any" | "qualified" } },
    );

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(api.getHotLeads).toHaveBeenLastCalledWith({ status: "any" });

    rerender({ status: "qualified" });
    await waitFor(() => expect(api.getHotLeads).toHaveBeenLastCalledWith({ status: "qualified" }));
  });

  it("surfaces error and keeps last good leads", async () => {
    vi.mocked(api.getHotLeads).mockResolvedValueOnce(response());
    vi.mocked(api.getHotLeads).mockRejectedValueOnce(new Error("network"));

    const { result } = renderHook(() => useHotLeads({}));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.leads).toHaveLength(1);

    await act(async () => {
      await result.current.refresh();
    });
    expect(result.current.error).not.toBeNull();
    expect(result.current.leads).toHaveLength(1); // last good preserved
  });
});
