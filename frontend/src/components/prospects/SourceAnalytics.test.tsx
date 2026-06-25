import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import type { SourceStatItem } from "@/lib/api";
import { SourceAnalytics } from "./SourceAnalytics";

function stat(over: Partial<SourceStatItem> = {}): SourceStatItem {
  return {
    source_id: "s1",
    source_name: "2GIS",
    category_name: "Каталоги",
    prospect_count: 8,
    lead_count: 2,
    converted_count: 5,
    ...over,
  };
}

describe("SourceAnalytics", () => {
  it("renders nothing when no source has prospects or leads", () => {
    const { container } = render(
      <SourceAnalytics stats={[stat({ prospect_count: 0, lead_count: 0, converted_count: 0 })]} />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("renders nothing for an empty stats array", () => {
    const { container } = render(<SourceAnalytics stats={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it("computes the conversion rate and shows the per-source counters", () => {
    // total = 8 + 2 = 10, converted 5 → 50%
    render(<SourceAnalytics stats={[stat()]} />);
    expect(screen.getByText("2GIS")).toBeInTheDocument();
    expect(screen.getByText("Каталоги")).toBeInTheDocument();
    expect(screen.getByText("50%")).toBeInTheDocument();
    expect(screen.getByText("8 просп.")).toBeInTheDocument();
    expect(screen.getByText("2 лидов")).toBeInTheDocument();
    expect(screen.getByText("5 конв.")).toBeInTheDocument();
  });

  it("caps the conversion bar width at 100% when conversions exceed the base", () => {
    // total = 1 + 0 = 1, converted 5 → 500% rounded, but bar width clamped to 100%
    render(<SourceAnalytics stats={[stat({ prospect_count: 1, lead_count: 0, converted_count: 5 })]} />);
    const fill = document.querySelector(".bg-\\[\\#004ac6\\]") as HTMLElement;
    expect(fill.style.width).toBe("100%");
  });

  it("filters out zero-activity sources but keeps active ones", () => {
    render(
      <SourceAnalytics
        stats={[
          stat({ source_id: "a", source_name: "Active", prospect_count: 3, lead_count: 0 }),
          stat({ source_id: "b", source_name: "Empty", prospect_count: 0, lead_count: 0, converted_count: 0 }),
        ]}
      />,
    );
    expect(screen.getByText("Active")).toBeInTheDocument();
    expect(screen.queryByText("Empty")).not.toBeInTheDocument();
  });
});
