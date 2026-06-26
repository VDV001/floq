import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import type { QualificationDistributionResponse } from "@/lib/api";
import { QualDistributionCard } from "./QualDistributionCard";

function data(over: Partial<QualificationDistributionResponse> = {}): QualificationDistributionResponse {
  return {
    step: 10,
    total: 8,
    buckets: [
      { lo: 0, hi: 9, label: "0–9", count: 1 },
      { lo: 10, hi: 19, label: "10–19", count: 0 },
      { lo: 90, hi: 100, label: "90–100", count: 5 },
    ],
    ...over,
  };
}

describe("QualDistributionCard", () => {
  it("renders every bucket label with its count and the total", () => {
    render(<QualDistributionCard data={data()} />);
    expect(screen.getByText("0–9")).toBeInTheDocument();
    expect(screen.getByText("90–100")).toBeInTheDocument();
    expect(screen.getByText("всего: 8")).toBeInTheDocument();
    expect(screen.getByTestId("bar-count-90")).toHaveTextContent("5");
    expect(screen.getByTestId("bar-count-10")).toHaveTextContent("0");
  });

  it("scales the tallest bucket to full width and others proportionally", () => {
    render(<QualDistributionCard data={data()} />);
    // tallest is lo=90 (count 5) → 100%; lo=0 (count 1) → 20%; lo=10 (0) → 0%.
    expect(screen.getByTestId("bar-fill-90")).toHaveStyle({ width: "100%" });
    expect(screen.getByTestId("bar-fill-0")).toHaveStyle({ width: "20%" });
    expect(screen.getByTestId("bar-fill-10")).toHaveStyle({ width: "0%" });
  });

  it("shows an empty state when there are no qualifications", () => {
    render(<QualDistributionCard data={data({ total: 0, buckets: [{ lo: 0, hi: 9, label: "0–9", count: 0 }] })} />);
    expect(screen.getByText("Пока нет квалифицированных лидов.")).toBeInTheDocument();
  });

  it("does not divide by zero when every bucket is empty (total > 0 guard off)", () => {
    // Defensive: all-zero counts but total reported non-zero must still
    // render 0%-width bars rather than NaN.
    render(
      <QualDistributionCard
        data={data({ total: 2, buckets: [{ lo: 0, hi: 9, label: "0–9", count: 0 }] })}
      />,
    );
    expect(screen.getByTestId("bar-fill-0")).toHaveStyle({ width: "0%" });
  });
});
