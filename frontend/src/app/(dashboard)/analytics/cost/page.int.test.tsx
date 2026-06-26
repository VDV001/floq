import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";

import { server, url } from "@/__tests__/integration/server";
import type { CostRatiosResponse, CostSummaryResponse } from "@/lib/api";

// AnalyticsTabs calls usePathname(); pin it so the page mounts deterministically
// without a Next router context.
vi.mock("next/navigation", () => ({
  usePathname: () => "/analytics/cost",
}));

import CostAnalyticsPage from "./page";

// Integration: real CostAnalyticsPage + useCostAnalytics + lib/api.ts, network via MSW.
// On mount the page fires exactly two GETs:
//   /api/analytics/cost-ratios?period=month  (summary card + ratio cards)
//   /api/audit/cost-summary?from=...&to=...   (breakdown tables)
const ratios: CostRatiosResponse = {
  period: { from: "2026-05-26", to: "2026-06-25" },
  total_cost_usd: 12.34,
  total_calls: 87,
  leads_count: 40,
  qualified_leads_count: 10,
  converted_count: 4,
  drafts_sent_count: 20,
  cost_per_lead_usd: 0.31,
  cost_per_qualified_lead_usd: 1.23,
  cost_per_converted_usd: 3.09,
  cost_per_draft_sent_usd: 0.62,
};

const summary: CostSummaryResponse = {
  total_usd: 12.34,
  total_calls: 87,
  period: { from: "2026-05-26", to: "2026-06-25" },
  by_request_type: [
    { request_type: "qualification", calls: 50, usd: 8.0, tokens_in: 1000, tokens_out: 500 },
    { request_type: "draft", calls: 37, usd: 4.5, tokens_in: 800, tokens_out: 400 },
  ],
  by_model: [{ model: "gpt-4o", calls: 87, usd: 9.99, tokens_in: 1800, tokens_out: 900 }],
};

function mountWith(extra: Parameters<typeof server.use> = []) {
  server.use(
    http.get(url("/api/analytics/cost-ratios"), () => HttpResponse.json(ratios)),
    http.get(url("/api/audit/cost-summary"), () => HttpResponse.json(summary)),
    ...extra,
  );
}

describe("cost analytics page (integration)", () => {
  it("loads data from the API and renders the summary, ratios and breakdowns", async () => {
    mountWith();

    render(<CostAnalyticsPage />);

    // Total cost from the cost-ratios response.
    expect(await screen.findByText("$12.34")).toBeInTheDocument();
    // A ratio card value (cost per converted) + its count hint.
    expect(screen.getByText("$3.09")).toBeInTheDocument();
    expect(screen.getByText("на 4 конверсий")).toBeInTheDocument();
    // Breakdown rows from the cost-summary response.
    expect(screen.getByText("qualification")).toBeInTheDocument();
    expect(screen.getByText("draft")).toBeInTheDocument();
    expect(screen.getByText("gpt-4o")).toBeInTheDocument();
  });

  it("re-sorts a breakdown table when a column header is clicked", async () => {
    const user = userEvent.setup({ delay: null });
    mountWith();

    render(<CostAnalyticsPage />);
    await screen.findByText("qualification");

    // Default sort is by USD descending: qualification ($8.00) before draft ($4.50).
    const labelCellsBefore = screen.getAllByText(/^(qualification|draft)$/);
    expect(labelCellsBefore[0]).toHaveTextContent("qualification");

    // Click "Тип запроса" twice -> label ascending: draft (< qualification) first.
    const labelHeader = screen.getByRole("button", { name: /Тип запроса/ });
    await user.click(labelHeader); // label desc
    await user.click(labelHeader); // label asc

    const labelCellsAfter = screen.getAllByText(/^(qualification|draft)$/);
    expect(labelCellsAfter[0]).toHaveTextContent("draft");
  });
});
