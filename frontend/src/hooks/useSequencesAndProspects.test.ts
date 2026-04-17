import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";

vi.mock("@/lib/api", () => ({
  api: {
    getProspects: vi.fn(),
    getSequences: vi.fn(),
    getSequence: vi.fn(),
    createSequence: vi.fn(),
    deleteSequence: vi.fn(),
    toggleSequence: vi.fn(),
    updateSequence: vi.fn(),
    launchSequence: vi.fn(),
    addStep: vi.fn(),
    deleteStep: vi.fn(),
  },
}));
import { api } from "@/lib/api";
import type { Prospect, Sequence, SequenceStep } from "@/lib/api";
import { useProspects } from "./useProspects";
import { useSequences } from "./useSequences";
import { useSequenceSteps } from "./useSequenceSteps";

// --- Factories ---

function makeProspect(overrides: Partial<Prospect> = {}): Prospect {
  return {
    id: overrides.id ?? "p-1",
    user_id: "u-1",
    name: "Test Prospect",
    company: "ACME",
    title: "CTO",
    email: "test@acme.com",
    phone: "",
    whatsapp: "",
    telegram_username: "",
    industry: "SaaS",
    company_size: "50-100",
    context: "",
    source: "csv",
    status: "new",
    verify_status: "not_checked",
    verify_score: 0,
    verify_details: {},
    verified_at: null,
    converted_lead_id: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

function makeSequence(overrides: Partial<Sequence> = {}): Sequence {
  return {
    id: overrides.id ?? "seq-1",
    user_id: "u-1",
    name: "Cold outreach",
    is_active: true,
    created_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

function makeStep(overrides: Partial<SequenceStep> = {}): SequenceStep {
  return {
    id: overrides.id ?? "step-1",
    sequence_id: "seq-1",
    step_order: 1,
    delay_days: 0,
    prompt_hint: "Intro email",
    channel: "email",
    created_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

const mockedApi = vi.mocked(api);

beforeEach(() => {
  vi.resetAllMocks();
  mockedApi.getProspects.mockResolvedValue([]);
  mockedApi.getSequences.mockResolvedValue([]);
  mockedApi.getSequence.mockResolvedValue({ sequence: makeSequence(), steps: [] });
});

// ============================================================
// useProspects
// ============================================================

describe("useProspects", () => {
  it("loads prospects on mount", async () => {
    const data = [makeProspect({ id: "p-1" }), makeProspect({ id: "p-2" })];
    mockedApi.getProspects.mockResolvedValue(data);

    const { result } = renderHook(() => useProspects());

    await waitFor(() => {
      expect(result.current.prospects).toHaveLength(2);
    });
    expect(mockedApi.getProspects).toHaveBeenCalledOnce();
  });

  it("returns empty array when API fails on mount", async () => {
    mockedApi.getProspects.mockRejectedValue(new Error("network"));

    const { result } = renderHook(() => useProspects());

    // Should stay empty (error caught silently)
    await waitFor(() => {
      expect(mockedApi.getProspects).toHaveBeenCalledOnce();
    });
    expect(result.current.prospects).toEqual([]);
  });

  describe("newProspectsCount", () => {
    const cases = [
      { name: "all new", prospects: [makeProspect({ status: "new" }), makeProspect({ id: "p-2", status: "new" })], expected: 2 },
      { name: "mixed statuses", prospects: [makeProspect({ status: "new" }), makeProspect({ id: "p-2", status: "in_sequence" }), makeProspect({ id: "p-3", status: "replied" })], expected: 1 },
      { name: "none new", prospects: [makeProspect({ status: "in_sequence" }), makeProspect({ id: "p-2", status: "converted" })], expected: 0 },
      { name: "empty list", prospects: [], expected: 0 },
    ];

    it.each(cases)("$name → $expected", async ({ prospects, expected }) => {
      mockedApi.getProspects.mockResolvedValue(prospects);
      const { result } = renderHook(() => useProspects());

      await waitFor(() => {
        expect(result.current.newProspectsCount).toBe(expected);
      });
    });
  });

  describe("toggleProspect", () => {
    it("adds prospect to selection, then removes on second toggle", async () => {
      mockedApi.getProspects.mockResolvedValue([makeProspect({ id: "p-1" })]);
      const { result } = renderHook(() => useProspects());

      await waitFor(() => expect(result.current.prospects).toHaveLength(1));

      act(() => result.current.toggleProspect("p-1"));
      expect(result.current.selectedProspects.has("p-1")).toBe(true);

      act(() => result.current.toggleProspect("p-1"));
      expect(result.current.selectedProspects.has("p-1")).toBe(false);
    });

    it("supports multiple selections", async () => {
      mockedApi.getProspects.mockResolvedValue([]);
      const { result } = renderHook(() => useProspects());

      await waitFor(() => expect(mockedApi.getProspects).toHaveBeenCalled());

      act(() => {
        result.current.toggleProspect("a");
        result.current.toggleProspect("b");
      });
      expect(result.current.selectedProspects.size).toBe(2);
      expect(result.current.selectedProspects.has("a")).toBe(true);
      expect(result.current.selectedProspects.has("b")).toBe(true);
    });
  });

  describe("selectAllProspects", () => {
    it("selects all when none selected", async () => {
      const data = [makeProspect({ id: "p-1" }), makeProspect({ id: "p-2" }), makeProspect({ id: "p-3" })];
      mockedApi.getProspects.mockResolvedValue(data);
      const { result } = renderHook(() => useProspects());

      await waitFor(() => expect(result.current.prospects).toHaveLength(3));

      act(() => result.current.selectAllProspects());
      expect(result.current.selectedProspects.size).toBe(3);
    });

    it("deselects all when all selected", async () => {
      const data = [makeProspect({ id: "p-1" }), makeProspect({ id: "p-2" })];
      mockedApi.getProspects.mockResolvedValue(data);
      const { result } = renderHook(() => useProspects());

      await waitFor(() => expect(result.current.prospects).toHaveLength(2));

      act(() => result.current.selectAllProspects());
      expect(result.current.selectedProspects.size).toBe(2);

      act(() => result.current.selectAllProspects());
      expect(result.current.selectedProspects.size).toBe(0);
    });

    it("selects all when only some selected (partial → all)", async () => {
      const data = [makeProspect({ id: "p-1" }), makeProspect({ id: "p-2" }), makeProspect({ id: "p-3" })];
      mockedApi.getProspects.mockResolvedValue(data);
      const { result } = renderHook(() => useProspects());

      await waitFor(() => expect(result.current.prospects).toHaveLength(3));

      act(() => result.current.toggleProspect("p-1"));
      expect(result.current.selectedProspects.size).toBe(1);

      act(() => result.current.selectAllProspects());
      expect(result.current.selectedProspects.size).toBe(3);
    });
  });

  describe("launchSequence", () => {
    it("calls API, clears selection, sets result message on success", async () => {
      const data = [makeProspect({ id: "p-1" }), makeProspect({ id: "p-2" })];
      mockedApi.getProspects.mockResolvedValue(data);
      mockedApi.launchSequence.mockResolvedValue(undefined);

      const { result } = renderHook(() => useProspects());

      await waitFor(() => expect(result.current.prospects).toHaveLength(2));

      act(() => {
        result.current.toggleProspect("p-1");
        result.current.toggleProspect("p-2");
      });
      expect(result.current.selectedProspects.size).toBe(2);

      await act(async () => {
        await result.current.launchSequence("seq-1", ["p-1", "p-2"], true);
      });

      expect(mockedApi.launchSequence).toHaveBeenCalledWith("seq-1", ["p-1", "p-2"], true);
      expect(result.current.selectedProspects.size).toBe(0);
      expect(result.current.launchResult).toBe("Запущено для 2 проспектов");
      expect(result.current.launching).toBe(false);
    });

    it("sets error message on failure", async () => {
      mockedApi.getProspects.mockResolvedValue([]);
      mockedApi.launchSequence.mockRejectedValue(new Error("fail"));

      const { result } = renderHook(() => useProspects());

      await waitFor(() => expect(mockedApi.getProspects).toHaveBeenCalled());

      await act(async () => {
        await result.current.launchSequence("seq-1", ["p-1"], false);
      });

      expect(result.current.launchResult).toBe("Ошибка запуска");
      expect(result.current.launching).toBe(false);
    });

    it("clears launchResult after 4 seconds", async () => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
      mockedApi.getProspects.mockResolvedValue([]);
      mockedApi.launchSequence.mockResolvedValue(undefined);

      const { result } = renderHook(() => useProspects());

      await waitFor(() => expect(mockedApi.getProspects).toHaveBeenCalled());

      await act(async () => {
        await result.current.launchSequence("seq-1", ["p-1"], true);
      });

      expect(result.current.launchResult).toBe("Запущено для 1 проспектов");

      await act(async () => {
        vi.advanceTimersByTime(4000);
      });

      expect(result.current.launchResult).toBeNull();
      vi.useRealTimers();
    });

    it("reloads prospects after successful launch", async () => {
      mockedApi.getProspects.mockResolvedValue([makeProspect({ id: "p-1", status: "new" })]);
      mockedApi.launchSequence.mockResolvedValue(undefined);

      const { result } = renderHook(() => useProspects());

      await waitFor(() => expect(result.current.prospects).toHaveLength(1));

      // After launch, getProspects is called again
      mockedApi.getProspects.mockResolvedValue([makeProspect({ id: "p-1", status: "in_sequence" })]);

      await act(async () => {
        await result.current.launchSequence("seq-1", ["p-1"], true);
      });

      // Initial load + reload after launch
      expect(mockedApi.getProspects).toHaveBeenCalledTimes(2);
    });
  });
});

// ============================================================
// useSequences
// ============================================================

describe("useSequences", () => {
  it("loads sequences on mount and auto-selects first", async () => {
    const data = [makeSequence({ id: "seq-1" }), makeSequence({ id: "seq-2" })];
    mockedApi.getSequences.mockResolvedValue(data);

    const { result } = renderHook(() => useSequences());

    expect(result.current.loading).toBe(true);

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.sequences).toHaveLength(2);
    expect(result.current.selectedSeqId).toBe("seq-1");
    expect(result.current.selectedSequence).toEqual(data[0]);
  });

  it("does not auto-select when no sequences returned", async () => {
    mockedApi.getSequences.mockResolvedValue([]);

    const { result } = renderHook(() => useSequences());

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.selectedSeqId).toBeNull();
    expect(result.current.selectedSequence).toBeNull();
  });

  it("sets loading=false even on API error", async () => {
    mockedApi.getSequences.mockRejectedValue(new Error("fail"));

    const { result } = renderHook(() => useSequences());

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.sequences).toEqual([]);
  });

  describe("createSequence", () => {
    it("adds new sequence to list and selects it", async () => {
      mockedApi.getSequences.mockResolvedValue([makeSequence({ id: "seq-1" })]);
      const newSeq = makeSequence({ id: "seq-new", name: "New Seq" });
      mockedApi.createSequence.mockResolvedValue(newSeq);

      const { result } = renderHook(() => useSequences());
      await waitFor(() => expect(result.current.loading).toBe(false));

      await act(async () => {
        await result.current.createSequence("New Seq");
      });

      expect(mockedApi.createSequence).toHaveBeenCalledWith("New Seq");
      expect(result.current.sequences).toHaveLength(2);
      expect(result.current.selectedSeqId).toBe("seq-new");
      expect(result.current.selectedSequence).toEqual(newSeq);
    });
  });

  describe("deleteSequence", () => {
    it("removes sequence from list", async () => {
      mockedApi.getSequences.mockResolvedValue([makeSequence({ id: "seq-1" }), makeSequence({ id: "seq-2" })]);
      mockedApi.deleteSequence.mockResolvedValue(undefined);

      const { result } = renderHook(() => useSequences());
      await waitFor(() => expect(result.current.loading).toBe(false));

      await act(async () => {
        await result.current.deleteSequence("seq-2");
      });

      expect(result.current.sequences).toHaveLength(1);
      expect(result.current.sequences[0].id).toBe("seq-1");
    });

    it("clears selection when deleting the selected sequence", async () => {
      mockedApi.getSequences.mockResolvedValue([makeSequence({ id: "seq-1" }), makeSequence({ id: "seq-2" })]);
      mockedApi.deleteSequence.mockResolvedValue(undefined);

      const { result } = renderHook(() => useSequences());
      await waitFor(() => expect(result.current.loading).toBe(false));

      // seq-1 is auto-selected
      expect(result.current.selectedSeqId).toBe("seq-1");

      await act(async () => {
        await result.current.deleteSequence("seq-1");
      });

      expect(result.current.selectedSeqId).toBeNull();
      expect(result.current.selectedSequence).toBeNull();
    });

    it("keeps selection when deleting a non-selected sequence", async () => {
      mockedApi.getSequences.mockResolvedValue([makeSequence({ id: "seq-1" }), makeSequence({ id: "seq-2" })]);
      mockedApi.deleteSequence.mockResolvedValue(undefined);

      const { result } = renderHook(() => useSequences());
      await waitFor(() => expect(result.current.loading).toBe(false));

      expect(result.current.selectedSeqId).toBe("seq-1");

      await act(async () => {
        await result.current.deleteSequence("seq-2");
      });

      expect(result.current.selectedSeqId).toBe("seq-1");
    });
  });

  describe("toggleSequence", () => {
    const cases = [
      { name: "activate", initial: false, target: true },
      { name: "deactivate", initial: true, target: false },
    ];

    it.each(cases)("$name: sets is_active to $target", async ({ initial, target }) => {
      mockedApi.getSequences.mockResolvedValue([makeSequence({ id: "seq-1", is_active: initial })]);
      mockedApi.toggleSequence.mockResolvedValue(undefined);

      const { result } = renderHook(() => useSequences());
      await waitFor(() => expect(result.current.loading).toBe(false));

      await act(async () => {
        await result.current.toggleSequence("seq-1", target);
      });

      expect(mockedApi.toggleSequence).toHaveBeenCalledWith("seq-1", target);
      expect(result.current.sequences[0].is_active).toBe(target);
    });
  });

  describe("renameSequence", () => {
    it("updates the sequence name in list", async () => {
      mockedApi.getSequences.mockResolvedValue([makeSequence({ id: "seq-1", name: "Old" })]);
      mockedApi.updateSequence.mockResolvedValue({ ...makeSequence({ id: "seq-1" }), name: "New Name" });

      const { result } = renderHook(() => useSequences());
      await waitFor(() => expect(result.current.loading).toBe(false));

      await act(async () => {
        await result.current.renameSequence("seq-1", "New Name");
      });

      expect(mockedApi.updateSequence).toHaveBeenCalledWith("seq-1", "New Name");
      expect(result.current.sequences[0].name).toBe("New Name");
    });
  });

  describe("selectedSequence derivation", () => {
    it("returns null when selectedSeqId does not match any sequence", async () => {
      mockedApi.getSequences.mockResolvedValue([makeSequence({ id: "seq-1" })]);

      const { result } = renderHook(() => useSequences());
      await waitFor(() => expect(result.current.loading).toBe(false));

      act(() => result.current.setSelectedSeqId("nonexistent"));

      expect(result.current.selectedSequence).toBeNull();
    });

    it("updates when setSelectedSeqId is called", async () => {
      mockedApi.getSequences.mockResolvedValue([makeSequence({ id: "seq-1" }), makeSequence({ id: "seq-2", name: "Second" })]);

      const { result } = renderHook(() => useSequences());
      await waitFor(() => expect(result.current.loading).toBe(false));

      act(() => result.current.setSelectedSeqId("seq-2"));

      expect(result.current.selectedSequence?.id).toBe("seq-2");
      expect(result.current.selectedSequence?.name).toBe("Second");
    });
  });
});

// ============================================================
// useSequenceSteps
// ============================================================

describe("useSequenceSteps", () => {
  it("clears steps when selectedSeqId is null", async () => {
    const { result } = renderHook(() => useSequenceSteps(null));

    await waitFor(() => {
      expect(result.current.steps).toEqual([]);
    });
    expect(mockedApi.getSequence).not.toHaveBeenCalled();
  });

  it("loads steps from api.getSequence when selectedSeqId is set", async () => {
    const steps = [makeStep({ id: "s-1" }), makeStep({ id: "s-2", step_order: 2 })];
    mockedApi.getSequence.mockResolvedValue({ sequence: makeSequence(), steps });

    const { result } = renderHook(() => useSequenceSteps("seq-1"));

    await waitFor(() => {
      expect(result.current.steps).toHaveLength(2);
    });
    expect(result.current.steps[0].id).toBe("s-1");
    expect(result.current.steps[1].id).toBe("s-2");
    expect(mockedApi.getSequence).toHaveBeenCalledWith("seq-1");
  });

  it("clears steps when selectedSeqId changes from value to null", async () => {
    const steps = [makeStep({ id: "s-1" })];
    mockedApi.getSequence.mockResolvedValue({ sequence: makeSequence(), steps });

    const { result, rerender } = renderHook(
      ({ seqId }: { seqId: string | null }) => useSequenceSteps(seqId),
      { initialProps: { seqId: "seq-1" as string | null } }
    );

    await waitFor(() => expect(result.current.steps).toHaveLength(1));

    rerender({ seqId: null as string | null });

    await waitFor(() => {
      expect(result.current.steps).toEqual([]);
    });
  });

  it("sets steps to empty on API error", async () => {
    mockedApi.getSequence.mockRejectedValue(new Error("fail"));

    const { result } = renderHook(() => useSequenceSteps("seq-1"));

    await waitFor(() => {
      expect(result.current.stepsLoading).toBe(false);
    });
    expect(result.current.steps).toEqual([]);
  });

  it("handles null steps field in response", async () => {
    mockedApi.getSequence.mockResolvedValue({ sequence: makeSequence(), steps: null as unknown as SequenceStep[] });

    const { result } = renderHook(() => useSequenceSteps("seq-1"));

    await waitFor(() => {
      expect(result.current.steps).toEqual([]);
    });
  });

  describe("addStep", () => {
    it("calls API with correct params and reloads steps", async () => {
      const initialSteps = [makeStep({ id: "s-1" })];
      const updatedSteps = [makeStep({ id: "s-1" }), makeStep({ id: "s-2", step_order: 2 })];

      mockedApi.getSequence
        .mockResolvedValueOnce({ sequence: makeSequence(), steps: initialSteps })
        .mockResolvedValueOnce({ sequence: makeSequence(), steps: updatedSteps });
      mockedApi.addStep.mockResolvedValue(undefined as unknown as SequenceStep);

      const { result } = renderHook(() => useSequenceSteps("seq-1"));

      await waitFor(() => expect(result.current.steps).toHaveLength(1));

      await act(async () => {
        await result.current.addStep({ channel: "email", delay_days: 3, prompt_hint: "Follow up" });
      });

      expect(mockedApi.addStep).toHaveBeenCalledWith("seq-1", {
        step_order: 2,
        delay_days: 3,
        prompt_hint: "Follow up",
        channel: "email",
      });
      expect(result.current.steps).toHaveLength(2);
    });

    it("does nothing when selectedSeqId is null", async () => {
      const { result } = renderHook(() => useSequenceSteps(null));

      await waitFor(() => expect(result.current.steps).toEqual([]));

      await act(async () => {
        await result.current.addStep({ channel: "telegram", delay_days: 1, prompt_hint: "hi" });
      });

      expect(mockedApi.addStep).not.toHaveBeenCalled();
    });

    const channelCases: Array<{ channel: "email" | "telegram"; delay: number }> = [
      { channel: "email", delay: 0 },
      { channel: "telegram", delay: 5 },
      { channel: "email", delay: 14 },
    ];

    it.each(channelCases)("passes channel=$channel, delay_days=$delay correctly", async ({ channel, delay }) => {
      mockedApi.getSequence.mockResolvedValue({ sequence: makeSequence(), steps: [] });
      mockedApi.addStep.mockResolvedValue(undefined as unknown as SequenceStep);

      const { result } = renderHook(() => useSequenceSteps("seq-1"));
      await waitFor(() => expect(mockedApi.getSequence).toHaveBeenCalled());

      await act(async () => {
        await result.current.addStep({ channel, delay_days: delay, prompt_hint: "test" });
      });

      expect(mockedApi.addStep).toHaveBeenCalledWith("seq-1", {
        step_order: 1,
        delay_days: delay,
        prompt_hint: "test",
        channel,
      });
    });
  });

  describe("deleteStep", () => {
    it("calls API then reloads steps", async () => {
      const initialSteps = [makeStep({ id: "s-1" }), makeStep({ id: "s-2", step_order: 2 })];
      const afterDelete = [makeStep({ id: "s-2", step_order: 1 })];

      mockedApi.getSequence
        .mockResolvedValueOnce({ sequence: makeSequence(), steps: initialSteps })
        .mockResolvedValueOnce({ sequence: makeSequence(), steps: afterDelete });
      mockedApi.deleteStep.mockResolvedValue(undefined as unknown as void);

      const { result } = renderHook(() => useSequenceSteps("seq-1"));

      await waitFor(() => expect(result.current.steps).toHaveLength(2));

      await act(async () => {
        await result.current.deleteStep("s-1");
      });

      expect(mockedApi.deleteStep).toHaveBeenCalledWith("seq-1", "s-1");
      expect(result.current.steps).toHaveLength(1);
      expect(result.current.steps[0].id).toBe("s-2");
    });

    it("does nothing when selectedSeqId is null", async () => {
      const { result } = renderHook(() => useSequenceSteps(null));

      await waitFor(() => expect(result.current.steps).toEqual([]));

      await act(async () => {
        await result.current.deleteStep("s-1");
      });

      expect(mockedApi.deleteStep).not.toHaveBeenCalled();
    });
  });

  describe("stepsLoading state", () => {
    it("is true while loading, false after", async () => {
      type SeqResponse = { sequence: Sequence; steps: SequenceStep[] };
      let resolveGetSequence!: (v: SeqResponse) => void;
      mockedApi.getSequence.mockImplementation(
        () => new Promise<SeqResponse>((resolve) => { resolveGetSequence = resolve; })
      );

      const { result } = renderHook(() => useSequenceSteps("seq-1"));

      await waitFor(() => expect(result.current.stepsLoading).toBe(true));

      await act(async () => {
        resolveGetSequence({ sequence: makeSequence(), steps: [makeStep()] });
      });

      await waitFor(() => expect(result.current.stepsLoading).toBe(false));
      expect(result.current.steps).toHaveLength(1);
    });
  });
});
