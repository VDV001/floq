import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";

vi.mock("@/lib/api", () => ({
  api: {
    getProspects: vi.fn(),
    getSourceStats: vi.fn(),
    verifyBatch: vi.fn(),
    setProspectConsent: vi.fn(),
  },
}));

import { api, type Prospect } from "@/lib/api";
import { useProspectsPage } from "./useProspectsPage";

type RawProspect = Parameters<typeof import("@/components/prospects/constants").mapProspects>[0][number];

function raw(over: Partial<RawProspect> = {}): Prospect {
  return {
    id: "p1",
    name: "Иван Петров",
    company: "Acme",
    title: "CTO",
    email: "ivan@acme.ru",
    phone: "+700",
    whatsapp: "",
    telegram_username: "",
    source_name: "2GIS",
    status: "new",
    consent_status: "none",
    verify_status: "not_checked",
    verify_score: 0,
    ...over,
  } as unknown as Prospect;
}

describe("useProspectsPage", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(api.getSourceStats).mockResolvedValue([]);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("loads and maps prospects on mount", async () => {
    vi.mocked(api.getProspects).mockResolvedValue([raw()]);
    const { result } = renderHook(() => useProspectsPage());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.prospects).toHaveLength(1);
    expect(result.current.prospects[0].name).toBe("Иван Петров");
  });

  it("keeps current prospects when the fetch rejects", async () => {
    vi.mocked(api.getProspects).mockRejectedValue(new Error("boom"));
    const { result } = renderHook(() => useProspectsPage());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.prospects).toHaveLength(0);
  });

  it("derives a sorted unique source-name list", async () => {
    vi.mocked(api.getProspects).mockResolvedValue([
      raw({ id: "a", source_name: "Яндекс" }),
      raw({ id: "b", source_name: "2GIS" }),
      raw({ id: "c", source_name: "2GIS" }),
    ]);
    const { result } = renderHook(() => useProspectsPage());
    await waitFor(() => expect(result.current.prospects).toHaveLength(3));
    expect(result.current.sourceNames).toEqual(["2GIS", "Яндекс"]);
  });

  it("filters by source name", async () => {
    vi.mocked(api.getProspects).mockResolvedValue([
      raw({ id: "a", source_name: "2GIS" }),
      raw({ id: "b", source_name: "Яндекс" }),
    ]);
    const { result } = renderHook(() => useProspectsPage());
    await waitFor(() => expect(result.current.prospects).toHaveLength(2));
    act(() => result.current.setSourceFilter("2GIS"));
    expect(result.current.filteredProspects).toHaveLength(1);
    expect(result.current.filteredProspects[0].sourceName).toBe("2GIS");
  });

  it("searches across name, company, email and position", async () => {
    vi.mocked(api.getProspects).mockResolvedValue([
      raw({ id: "a", name: "Алиса", company: "Beta", email: "x@y.z", title: "Dev" }),
      raw({ id: "b", name: "Боб", company: "Gamma", email: "bob@corp.io", title: "Lead" }),
    ]);
    const { result } = renderHook(() => useProspectsPage());
    await waitFor(() => expect(result.current.prospects).toHaveLength(2));

    act(() => result.current.setSearchQuery("алиса"));
    expect(result.current.filteredProspects.map((p) => p.id)).toEqual(["a"]);

    act(() => result.current.setSearchQuery("gamma"));
    expect(result.current.filteredProspects.map((p) => p.id)).toEqual(["b"]);

    act(() => result.current.setSearchQuery("bob@corp"));
    expect(result.current.filteredProspects.map((p) => p.id)).toEqual(["b"]);

    act(() => result.current.setSearchQuery("lead"));
    expect(result.current.filteredProspects.map((p) => p.id)).toEqual(["b"]);

    act(() => result.current.setSearchQuery("nomatch"));
    expect(result.current.filteredProspects).toHaveLength(0);
  });

  it("paginates and clamps the page to the available range", async () => {
    const many = Array.from({ length: 20 }, (_, i) => raw({ id: `p${i}`, name: `Name ${i}` }));
    vi.mocked(api.getProspects).mockResolvedValue(many);
    const { result } = renderHook(() => useProspectsPage());
    await waitFor(() => expect(result.current.prospects).toHaveLength(20));

    expect(result.current.totalPages).toBe(2);
    expect(result.current.pagedProspects).toHaveLength(15);
    expect(result.current.rangeStart).toBe(1);
    expect(result.current.rangeEnd).toBe(15);

    act(() => result.current.setPage(2));
    expect(result.current.pagedProspects).toHaveLength(5);
    expect(result.current.rangeStart).toBe(16);
    expect(result.current.rangeEnd).toBe(20);

    // Clamp: asking for a page beyond the range falls back to the last page.
    act(() => result.current.setPage(99));
    expect(result.current.page).toBe(2);
  });

  it("shows a success toast after verifying a positive batch", async () => {
    vi.useFakeTimers();
    vi.mocked(api.getProspects).mockResolvedValue([raw()]);
    vi.mocked(api.verifyBatch).mockResolvedValue({ verified: 3 } as never);
    const { result } = renderHook(() => useProspectsPage());
    await vi.advanceTimersByTimeAsync(0);

    let p: Promise<void>;
    act(() => { p = result.current.handleVerifyBatch(); });
    await act(async () => { await vi.advanceTimersByTimeAsync(2500); await p; });
    expect(result.current.toast).toEqual({ message: "Проверено 3 проспектов", type: "success" });
    expect(result.current.verifying).toBe(false);
  });

  it("shows an error toast when no prospects were verified", async () => {
    vi.useFakeTimers();
    vi.mocked(api.getProspects).mockResolvedValue([raw()]);
    vi.mocked(api.verifyBatch).mockResolvedValue({ verified: 0 } as never);
    const { result } = renderHook(() => useProspectsPage());
    await vi.advanceTimersByTimeAsync(0);

    let p: Promise<void>;
    act(() => { p = result.current.handleVerifyBatch(); });
    await act(async () => { await vi.advanceTimersByTimeAsync(2500); await p; });
    expect(result.current.toast).toEqual({ message: "Нет проспектов для проверки", type: "error" });
  });

  it("shows an error toast when the verify call throws", async () => {
    vi.useFakeTimers();
    vi.mocked(api.getProspects).mockResolvedValue([raw()]);
    vi.mocked(api.verifyBatch).mockRejectedValue(new Error("down"));
    const { result } = renderHook(() => useProspectsPage());
    await vi.advanceTimersByTimeAsync(0);

    let p: Promise<void>;
    act(() => { p = result.current.handleVerifyBatch(); });
    await act(async () => { await vi.advanceTimersByTimeAsync(2500); await p; });
    expect(result.current.toast).toEqual({ message: "Ошибка проверки", type: "error" });
  });

  it("marks consent obtained and withdrawn, and reports failures", async () => {
    vi.mocked(api.getProspects).mockResolvedValue([raw()]);
    vi.mocked(api.setProspectConsent).mockResolvedValue(undefined as never);
    const { result } = renderHook(() => useProspectsPage());
    await waitFor(() => expect(result.current.prospects).toHaveLength(1));

    await act(async () => { await result.current.handleSetConsent("p1", "obtained"); });
    expect(result.current.toast).toEqual({ message: "Согласие отмечено", type: "success" });

    await act(async () => { await result.current.handleSetConsent("p1", "withdrawn"); });
    expect(result.current.toast).toEqual({ message: "Согласие отозвано", type: "success" });

    vi.mocked(api.setProspectConsent).mockRejectedValueOnce(new Error("nope"));
    await act(async () => { await result.current.handleSetConsent("p1", "obtained"); });
    expect(result.current.toast).toEqual({ message: "Не удалось изменить согласие", type: "error" });
  });
});
