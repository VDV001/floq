import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { CostRatiosResponse, CostSummaryResponse } from "@/lib/api";

vi.mock("next/navigation", () => ({
  usePathname: () => "/analytics/cost",
  useRouter: () => ({ push: vi.fn() }),
}));

interface HookState {
  ratios: CostRatiosResponse | null;
  summary: CostSummaryResponse | null;
  loading: boolean;
  error: Error | null;
  lastUpdated: Date | null;
  refresh: ReturnType<typeof vi.fn>;
}

const hookState: HookState = {
  ratios: null,
  summary: null,
  loading: false,
  error: null,
  lastUpdated: null,
  refresh: vi.fn(),
};

const useCostAnalytics = vi.fn((_period: string) => hookState);
vi.mock("@/hooks/useCostAnalytics", () => ({
  useCostAnalytics: (period: string) => useCostAnalytics(period),
}));

import CostAnalyticsPage from "./page";

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
  ],
  by_model: [{ model: "gpt-4o", calls: 87, usd: 9.99, tokens_in: 1800, tokens_out: 900 }],
};

beforeEach(() => {
  hookState.ratios = null;
  hookState.summary = null;
  hookState.loading = false;
  hookState.error = null;
  hookState.lastUpdated = null;
  hookState.refresh = vi.fn();
  useCostAnalytics.mockClear();
});

describe("CostAnalyticsPage (unit)", () => {
  it("renders the loading placeholder while loading and no ratios yet", () => {
    hookState.loading = true;
    render(<CostAnalyticsPage />);
    expect(screen.getByText("Загружаем…")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Аналитика затрат" })).toBeInTheDocument();
  });

  it("renders an error banner when the hook reports an error", () => {
    hookState.error = new Error("boom");
    render(<CostAnalyticsPage />);
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Не удалось загрузить данные: boom");
  });

  it("renders neither loading nor cards in the empty state (no ratios, not loading)", () => {
    render(<CostAnalyticsPage />);
    expect(screen.queryByText("Загружаем…")).not.toBeInTheDocument();
    expect(screen.queryByText("AI-расход за период")).not.toBeInTheDocument();
  });

  it("renders the summary card and breakdown tables when ratios + summary are present", () => {
    hookState.ratios = ratios;
    hookState.summary = summary;
    hookState.lastUpdated = new Date("2026-06-25T10:00:00Z");
    render(<CostAnalyticsPage />);
    expect(screen.getByText("$12.34")).toBeInTheDocument();
    expect(screen.getByText("qualification")).toBeInTheDocument();
    expect(screen.getByText("gpt-4o")).toBeInTheDocument();
    expect(screen.getByText(/Обновлено:/)).toBeInTheDocument();
    expect(screen.queryByText("Загружаем…")).not.toBeInTheDocument();
  });

  it("calls refresh when the Обновить button is clicked", () => {
    hookState.ratios = ratios;
    render(<CostAnalyticsPage />);
    fireEvent.click(screen.getByRole("button", { name: "Обновить" }));
    expect(hookState.refresh).toHaveBeenCalledTimes(1);
  });

  it("re-queries the hook with the new period when PeriodSelector changes", () => {
    render(<CostAnalyticsPage />);
    expect(useCostAnalytics).toHaveBeenCalledWith("month");
    fireEvent.click(screen.getByRole("radio", { name: "Неделя" }));
    expect(useCostAnalytics).toHaveBeenLastCalledWith("week");
  });
});
