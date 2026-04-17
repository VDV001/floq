import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AiTipCard } from "./AiTipCard";
import type { SequenceStep } from "@/lib/api";

function makeStep(overrides: Partial<SequenceStep> = {}): SequenceStep {
  return {
    id: "step-1",
    sequence_id: "seq-1",
    step_order: 1,
    delay_days: 0,
    prompt_hint: "Intro",
    channel: "email",
    created_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("AiTipCard", () => {
  it("shows create prompt when no sequences", () => {
    render(<AiTipCard sequenceCount={0} selectedSeqId={null} steps={[]} />);

    expect(screen.getByText(/Создайте первую секвенцию/)).toBeInTheDocument();
  });

  it("shows count when sequences exist", () => {
    render(<AiTipCard sequenceCount={3} selectedSeqId="s1" steps={[makeStep()]} />);

    expect(screen.getByText(/У вас 3 секвенций/)).toBeInTheDocument();
  });

  it("optimize alerts missing channels", async () => {
    const alertSpy = vi.spyOn(window, "alert").mockImplementation(() => {});
    const steps = [makeStep({ channel: "email" })];

    render(<AiTipCard sequenceCount={1} selectedSeqId="s1" steps={steps} />);

    await userEvent.click(screen.getByText(/Оптимизировать/));

    expect(alertSpy).toHaveBeenCalledWith(expect.stringContaining("Telegram"));
    alertSpy.mockRestore();
  });

  it("optimize alerts optimal when all channels used", async () => {
    const alertSpy = vi.spyOn(window, "alert").mockImplementation(() => {});
    const steps = [
      makeStep({ id: "1", channel: "email" }),
      makeStep({ id: "2", channel: "telegram" }),
      makeStep({ id: "3", channel: "phone_call" }),
    ];

    render(<AiTipCard sequenceCount={1} selectedSeqId="s1" steps={steps} />);

    await userEvent.click(screen.getByText(/Оптимизировать/));

    expect(alertSpy).toHaveBeenCalledWith(expect.stringContaining("оптимальна"));
    alertSpy.mockRestore();
  });

  it("optimize alerts when no sequence selected", async () => {
    const alertSpy = vi.spyOn(window, "alert").mockImplementation(() => {});

    render(<AiTipCard sequenceCount={1} selectedSeqId={null} steps={[]} />);

    await userEvent.click(screen.getByText(/Оптимизировать/));

    expect(alertSpy).toHaveBeenCalledWith(expect.stringContaining("Выберите секвенцию"));
    alertSpy.mockRestore();
  });
});
