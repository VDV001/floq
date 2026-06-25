import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import type { SequenceConversionResponse, SequenceStepConversion } from "@/lib/api";
import { SequenceConversionTable } from "./SequenceConversionTable";

function step(over: Partial<SequenceStepConversion> = {}): SequenceStepConversion {
  return {
    sequence_id: "s-1",
    sequence_name: "Warm intro",
    step_order: 1,
    entered: 10,
    replied: 4,
    advanced: 2,
    reply_rate: 0.4,
    advance_rate: 0.2,
    ...over,
  };
}

describe("SequenceConversionTable", () => {
  it("renders a row per step with counts and percent rates", () => {
    render(<SequenceConversionTable data={{ steps: [step()] }} />);
    expect(screen.getByText("Warm intro")).toBeInTheDocument();
    expect(screen.getByText("40%")).toBeInTheDocument(); // reply_rate
    expect(screen.getByText("20%")).toBeInTheDocument(); // advance_rate
  });

  it("rounds rates to whole percents", () => {
    // 0.005 -> round(0.5) -> 1%; 0.999 -> round(99.9) -> 100%.
    render(
      <SequenceConversionTable
        data={{ steps: [step({ reply_rate: 0.005, advance_rate: 0.999 })] }}
      />,
    );
    expect(screen.getByText("1%")).toBeInTheDocument();
    expect(screen.getByText("100%")).toBeInTheDocument();
  });

  it("shows an empty state when there are no steps", () => {
    const empty: SequenceConversionResponse = { steps: [] };
    render(<SequenceConversionTable data={empty} />);
    expect(screen.getByText("Пока нет отправленных шагов.")).toBeInTheDocument();
  });

  it("renders multiple sequences and steps in order", () => {
    render(
      <SequenceConversionTable
        data={{
          steps: [
            step({ sequence_name: "Warm intro", step_order: 1 }),
            step({ sequence_name: "Warm intro", step_order: 2, reply_rate: 0.1, advance_rate: 0 }),
          ],
        }}
      />,
    );
    const rows = screen.getAllByRole("row");
    // 1 header + 2 body rows
    expect(rows).toHaveLength(3);
  });
});
