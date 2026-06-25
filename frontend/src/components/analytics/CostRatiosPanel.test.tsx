import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import type { CostRatiosResponse } from "@/lib/api";
import { CostRatiosPanel } from "./CostRatiosPanel";

function ratios(over: Partial<CostRatiosResponse> = {}): CostRatiosResponse {
  return {
    period: { from: "2026-06-01", to: "2026-06-30" },
    total_cost_usd: 1,
    total_calls: 1,
    leads_count: 10,
    qualified_leads_count: 4,
    converted_count: 2,
    drafts_sent_count: 5,
    cost_per_lead_usd: 0.5,
    cost_per_qualified_lead_usd: 0.0042,
    cost_per_converted_usd: 0,
    cost_per_draft_sent_usd: 1.25,
    ...over,
  };
}

describe("CostRatiosPanel", () => {
  it("renders all four ratio cards with their labels", () => {
    render(<CostRatiosPanel ratios={ratios()} />);
    expect(screen.getByText("Стоимость / лид")).toBeInTheDocument();
    expect(screen.getByText("Стоимость / квалиф. лид")).toBeInTheDocument();
    expect(screen.getByText("Стоимость / конверсия")).toBeInTheDocument();
    expect(screen.getByText("Стоимость / отправл.")).toBeInTheDocument();
  });

  it("formats normal, sub-cent and zero ratios differently", () => {
    render(<CostRatiosPanel ratios={ratios()} />);
    expect(screen.getByText("$0.50")).toBeInTheDocument(); // normal
    expect(screen.getByText("$0.0042")).toBeInTheDocument(); // sub-cent
    expect(screen.getByText("—")).toBeInTheDocument(); // zero
  });

  it("shows the count hint when count is positive", () => {
    render(<CostRatiosPanel ratios={ratios({ leads_count: 10 })} />);
    expect(screen.getByText("на 10 лидов")).toBeInTheDocument();
  });

  it("shows the empty hint when count is zero", () => {
    render(<CostRatiosPanel ratios={ratios({ converted_count: 0 })} />);
    expect(screen.getByText("нет конверсий в периоде")).toBeInTheDocument();
  });
});
