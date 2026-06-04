import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import type { ScoreBucket } from "@/lib/api";
import { QualificationHistogram } from "./QualificationHistogram";

const bands: ScoreBucket[] = [
  { range: "0-20", count: 1 },
  { range: "21-40", count: 0 },
  { range: "41-60", count: 0 },
  { range: "61-80", count: 2 },
  { range: "81-100", count: 5 },
];

describe("QualificationHistogram", () => {
  it("renders every band label with its count", () => {
    render(<QualificationHistogram histogram={bands} avgScore={55} />);
    for (const b of bands) {
      expect(screen.getByText(b.range)).toBeInTheDocument();
    }
    expect(screen.getByTestId("bar-count-81-100")).toHaveTextContent("5");
    expect(screen.getByTestId("bar-count-21-40")).toHaveTextContent("0");
  });

  it("scales the tallest band to full width and others proportionally", () => {
    render(<QualificationHistogram histogram={bands} avgScore={55} />);
    // tallest is 81-100 (count 5) → 100%; 61-80 (count 2) → 40%.
    expect(screen.getByTestId("bar-fill-81-100")).toHaveStyle({ width: "100%" });
    expect(screen.getByTestId("bar-fill-61-80")).toHaveStyle({ width: "40%" });
    expect(screen.getByTestId("bar-fill-21-40")).toHaveStyle({ width: "0%" });
  });

  it("shows the average score", () => {
    render(<QualificationHistogram histogram={bands} avgScore={55.4} />);
    expect(screen.getByText(/55\.4/)).toBeInTheDocument();
  });

  it("renders an empty state when there are no qualifications", () => {
    const empty: ScoreBucket[] = bands.map((b) => ({ ...b, count: 0 }));
    render(<QualificationHistogram histogram={empty} avgScore={0} />);
    expect(screen.getByText(/Нет квалификаций/i)).toBeInTheDocument();
  });
});
