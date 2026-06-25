import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import type {
  QualificationDistributionResponse,
  SequenceConversionResponse,
} from "@/lib/api";

vi.mock("@/lib/api", () => ({
  api: {
    getQualificationDistribution: vi.fn(),
    getSequenceConversion: vi.fn(),
  },
}));

import { api } from "@/lib/api";
import { useFunnelAnalytics } from "./useFunnelAnalytics";

const dist: QualificationDistributionResponse = { step: 10, total: 0, buckets: [] };
const conv: SequenceConversionResponse = { steps: [] };

describe("useFunnelAnalytics", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(api.getQualificationDistribution).mockResolvedValue(dist);
    vi.mocked(api.getSequenceConversion).mockResolvedValue(conv);
  });

  it("fetches both read-models on mount with the default period (all)", async () => {
    const { result } = renderHook(() => useFunnelAnalytics());

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(api.getQualificationDistribution).toHaveBeenCalledWith("all");
    expect(api.getSequenceConversion).toHaveBeenCalledWith("all");
    expect(result.current.error).toBeNull();
  });

  it("refetches with the new period when it changes", async () => {
    const { result, rerender } = renderHook(
      ({ period }: { period: "week" | "month" | "all" }) => useFunnelAnalytics(period),
      { initialProps: { period: "all" as "week" | "month" | "all" } },
    );

    await waitFor(() => expect(result.current.loading).toBe(false));

    rerender({ period: "week" });

    await waitFor(() => {
      expect(api.getQualificationDistribution).toHaveBeenCalledWith("week");
      expect(api.getSequenceConversion).toHaveBeenCalledWith("week");
    });
  });

  it("surfaces an Error and preserves the last good read-models on a failed refresh", async () => {
    vi.mocked(api.getQualificationDistribution).mockResolvedValueOnce({ ...dist, total: 5 });
    vi.mocked(api.getSequenceConversion).mockResolvedValueOnce(conv);

    const { result } = renderHook(() => useFunnelAnalytics());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.distribution?.total).toBe(5);

    vi.mocked(api.getQualificationDistribution).mockRejectedValueOnce(new Error("matview down"));
    await act(async () => {
      await result.current.refresh();
    });
    expect(result.current.error).toEqual(new Error("matview down"));
    expect(result.current.distribution?.total).toBe(5); // last good preserved
  });

  it("wraps a non-Error rejection into an Error", async () => {
    const { result } = renderHook(() => useFunnelAnalytics());
    await waitFor(() => expect(result.current.loading).toBe(false));

    vi.mocked(api.getSequenceConversion).mockRejectedValueOnce("plain string failure");
    await act(async () => {
      await result.current.refresh();
    });
    expect(result.current.error).toBeInstanceOf(Error);
    expect(result.current.error?.message).toBe("plain string failure");
  });
});
