import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi, describe, it, expect } from "vitest";
import { StepTimeline } from "./StepTimeline";
import type { SequenceStep } from "@/lib/api";

// Isolate StepTimeline from its children (tested separately).
vi.mock("./StepPreview", () => ({
  StepPreview: ({ channel }: { channel: string }) => (
    <div data-testid="step-preview">preview:{channel}</div>
  ),
}));
vi.mock("./AddStepForm", () => ({
  AddStepForm: ({ onCancel }: { onCancel: () => void }) => (
    <div data-testid="add-step-form">
      <button onClick={onCancel}>cancel</button>
    </div>
  ),
}));

function step(over: Partial<SequenceStep> = {}): SequenceStep {
  return {
    id: "s1",
    sequence_id: "seq1",
    step_order: 1,
    channel: "email",
    delay_days: 0,
    prompt_hint: "первое касание",
    body: "",
    ...over,
  } as SequenceStep;
}

const noop = {
  onDeleteStep: vi.fn(),
  onAddStep: vi.fn().mockResolvedValue(undefined),
  onConfirmDelete: vi.fn(),
};

describe("StepTimeline", () => {
  it("shows a spinner while loading", () => {
    render(
      <StepTimeline
        selectedSeqId="seq1"
        selectedSequenceName="Test"
        steps={[]}
        stepsLoading
        {...noop}
      />,
    );
    expect(screen.queryByText("Выберите секвенцию слева")).not.toBeInTheDocument();
    expect(screen.queryByText("Нет шагов в этой секвенции")).not.toBeInTheDocument();
  });

  it("prompts to pick a sequence when none is selected", () => {
    render(
      <StepTimeline
        selectedSeqId={null}
        selectedSequenceName={null}
        steps={[]}
        stepsLoading={false}
        {...noop}
      />,
    );
    expect(screen.getByText("Выберите секвенцию слева")).toBeInTheDocument();
  });

  it("shows the empty state for a selected sequence with no steps", () => {
    render(
      <StepTimeline
        selectedSeqId="seq1"
        selectedSequenceName="Test"
        steps={[]}
        stepsLoading={false}
        {...noop}
      />,
    );
    expect(screen.getByText("Нет шагов в этой секвенции")).toBeInTheDocument();
  });

  it("renders steps with cumulative day numbers and first/delay labels", () => {
    render(
      <StepTimeline
        selectedSeqId="seq1"
        selectedSequenceName="Outreach"
        steps={[
          step({ id: "a", step_order: 1, delay_days: 0, channel: "email" }),
          step({ id: "b", step_order: 2, delay_days: 3, channel: "telegram", prompt_hint: "follow up" }),
        ]}
        stepsLoading={false}
        {...noop}
      />,
    );
    expect(screen.getByText(/Шаг 1/)).toBeInTheDocument();
    expect(screen.getByText(/Отправка сразу/)).toBeInTheDocument();
    expect(screen.getByText("Задержка: 3 дней")).toBeInTheDocument();
    expect(screen.getByText("День 0")).toBeInTheDocument(); // first step cumulative = 0
    expect(screen.getByText("День 3")).toBeInTheDocument(); // 0 + 3
    expect(screen.getByText("— Outreach")).toBeInTheDocument();
  });

  it("renders a manual body verbatim and hides the generate button", () => {
    render(
      <StepTimeline
        selectedSeqId="seq1"
        selectedSequenceName="T"
        steps={[step({ body: "Привет вручную" })]}
        stepsLoading={false}
        {...noop}
      />,
    );
    expect(screen.getByText("Написано вручную")).toBeInTheDocument();
    expect(screen.getByText("Привет вручную")).toBeInTheDocument();
    expect(screen.queryByText("Сгенерировать пример")).not.toBeInTheDocument();
  });

  it("opens the StepPreview for an AI step", async () => {
    render(
      <StepTimeline
        selectedSeqId="seq1"
        selectedSequenceName="T"
        steps={[step({ channel: "telegram", body: "" })]}
        stepsLoading={false}
        {...noop}
      />,
    );
    expect(screen.queryByTestId("step-preview")).not.toBeInTheDocument();
    await userEvent.click(screen.getByText("Сгенерировать пример"));
    expect(screen.getByTestId("step-preview")).toHaveTextContent("preview:telegram");
  });

  it("routes deletion through the confirm dialog, then deletes on confirm", async () => {
    const onDeleteStep = vi.fn();
    const onConfirmDelete = vi.fn();
    render(
      <StepTimeline
        selectedSeqId="seq1"
        selectedSequenceName="T"
        steps={[step({ id: "del-me", step_order: 2 })]}
        stepsLoading={false}
        onDeleteStep={onDeleteStep}
        onAddStep={noop.onAddStep}
        onConfirmDelete={onConfirmDelete}
      />,
    );
    // The trash button is the second action button in the row.
    const trash = screen.getAllByRole("button").find((b) => b.querySelector("svg.lucide-trash2"))
      ?? screen.getAllByRole("button")[1];
    await userEvent.click(trash);

    expect(onConfirmDelete).toHaveBeenCalledTimes(1);
    const [title, , onConfirm] = onConfirmDelete.mock.calls[0];
    expect(title).toBe("Удалить шаг");
    // Invoking the captured confirm callback performs the actual delete.
    onConfirm();
    expect(onDeleteStep).toHaveBeenCalledWith("del-me");
  });

  it("toggles the add-step form", async () => {
    render(
      <StepTimeline
        selectedSeqId="seq1"
        selectedSequenceName="T"
        steps={[]}
        stepsLoading={false}
        {...noop}
      />,
    );
    expect(screen.queryByTestId("add-step-form")).not.toBeInTheDocument();
    await userEvent.click(screen.getByText("Добавить шаг"));
    expect(screen.getByTestId("add-step-form")).toBeInTheDocument();
    await userEvent.click(screen.getByText("cancel"));
    expect(screen.queryByTestId("add-step-form")).not.toBeInTheDocument();
  });
});
