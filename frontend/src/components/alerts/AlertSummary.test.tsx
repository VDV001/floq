import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { AlertSummary } from "./AlertSummary";

describe("AlertSummary", () => {
  it("renders the critical and warning counts", () => {
    render(
      <AlertSummary followupCount={10} criticalCount={4} warningCount={6} />
    );
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(screen.getByText("6")).toBeInTheDocument();
    expect(screen.getByText("Сводка алертов")).toBeInTheDocument();
  });

  it("computes proportional bar widths when there are followups", () => {
    const { container } = render(
      <AlertSummary followupCount={10} criticalCount={4} warningCount={6} />
    );
    const bars = container.querySelectorAll<HTMLDivElement>(".h-full");
    expect(bars[0].style.width).toBe("40%");
    expect(bars[1].style.width).toBe("60%");
  });

  it("falls back to 0% widths when followupCount is zero", () => {
    const { container } = render(
      <AlertSummary followupCount={0} criticalCount={0} warningCount={0} />
    );
    const bars = container.querySelectorAll<HTMLDivElement>(".h-full");
    expect(bars[0].style.width).toBe("0%");
    expect(bars[1].style.width).toBe("0%");
  });
});
