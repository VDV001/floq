import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import type { CostRatiosResponse } from "@/lib/api";
import { CostSummaryCard } from "./CostSummaryCard";

function ratios(over: Partial<CostRatiosResponse> = {}): CostRatiosResponse {
  return {
    period: { from: "2026-06-01", to: "2026-06-30" },
    total_cost_usd: 12.3456,
    total_calls: 42,
    leads_count: 0,
    qualified_leads_count: 0,
    converted_count: 0,
    drafts_sent_count: 0,
    cost_per_lead_usd: 0,
    cost_per_qualified_lead_usd: 0,
    cost_per_converted_usd: 0,
    cost_per_draft_sent_usd: 0,
    ...over,
  };
}

describe("CostSummaryCard", () => {
  it("formats a normal cost with two decimals and shows the call count", () => {
    render(<CostSummaryCard ratios={ratios({ total_cost_usd: 12.3456, total_calls: 42 })} />);
    expect(screen.getByText("$12.35")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
  });

  it("formats sub-cent costs with four decimals", () => {
    render(<CostSummaryCard ratios={ratios({ total_cost_usd: 0.0042 })} />);
    expect(screen.getByText("$0.0042")).toBeInTheDocument();
  });

  it("renders a zero cost as $0.0000 (sub-cent branch)", () => {
    render(<CostSummaryCard ratios={ratios({ total_cost_usd: 0 })} />);
    expect(screen.getByText("$0.0000")).toBeInTheDocument();
  });
});
