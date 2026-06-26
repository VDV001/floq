import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { QualificationDistributionResponse, SequenceConversionResponse } from "@/lib/api";

vi.mock("next/navigation", () => ({
  usePathname: () => "/analytics/funnel",
  useRouter: () => ({ push: vi.fn() }),
}));

interface HookState {
  distribution: QualificationDistributionResponse | null;
  conversion: SequenceConversionResponse | null;
  loading: boolean;
  error: Error | null;
  lastUpdated: Date | null;
  refresh: ReturnType<typeof vi.fn>;
}

const hookState: HookState = {
  distribution: null,
  conversion: null,
  loading: false,
  error: null,
  lastUpdated: null,
  refresh: vi.fn(),
};

const useFunnelAnalytics = vi.fn((_period: string) => hookState);
vi.mock("@/hooks/useFunnelAnalytics", () => ({
  useFunnelAnalytics: (period: string) => useFunnelAnalytics(period),
}));

import FunnelAnalyticsPage from "./page";

const distribution: QualificationDistributionResponse = {
  step: 1,
  total: 12,
  buckets: [
    { lo: 80, hi: 100, label: "80–100", count: 7 },
    { lo: 50, hi: 79, label: "50–79", count: 5 },
  ],
};

const conversion: SequenceConversionResponse = {
  steps: [
    {
      sequence_id: "s1",
      sequence_name: "Welcome",
      step_order: 1,
      entered: 100,
      replied: 20,
      advanced: 15,
      reply_rate: 0.2,
      advance_rate: 0.15,
    },
  ],
};

beforeEach(() => {
  hookState.distribution = null;
  hookState.conversion = null;
  hookState.loading = false;
  hookState.error = null;
  hookState.lastUpdated = null;
  hookState.refresh = vi.fn();
  useFunnelAnalytics.mockClear();
});

describe("FunnelAnalyticsPage (unit)", () => {
  it("renders the loading placeholder while loading with no data yet", () => {
    hookState.loading = true;
    render(<FunnelAnalyticsPage />);
    expect(screen.getByText("Загружаем…")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Конверсия" })).toBeInTheDocument();
  });

  it("renders an error banner when the hook reports an error", () => {
    hookState.error = new Error("net down");
    render(<FunnelAnalyticsPage />);
    expect(screen.getByRole("alert")).toHaveTextContent("Не удалось загрузить данные: net down");
  });

  it("renders no cards in the empty state (no data, not loading)", () => {
    render(<FunnelAnalyticsPage />);
    expect(screen.queryByText("Загружаем…")).not.toBeInTheDocument();
    expect(screen.queryByText("Распределение скоров квалификации")).not.toBeInTheDocument();
  });

  it("renders distribution and conversion cards when data is present", () => {
    hookState.distribution = distribution;
    hookState.conversion = conversion;
    hookState.lastUpdated = new Date("2026-06-25T10:00:00Z");
    render(<FunnelAnalyticsPage />);
    expect(screen.getByText("Распределение скоров квалификации")).toBeInTheDocument();
    expect(screen.getByText("Welcome")).toBeInTheDocument();
    expect(screen.getByText(/Обновлено:/)).toBeInTheDocument();
  });

  it("calls refresh when the Обновить button is clicked", () => {
    render(<FunnelAnalyticsPage />);
    fireEvent.click(screen.getByRole("button", { name: "Обновить" }));
    expect(hookState.refresh).toHaveBeenCalledTimes(1);
  });

  it("re-queries the hook with the new period when PeriodSelector changes", () => {
    render(<FunnelAnalyticsPage />);
    expect(useFunnelAnalytics).toHaveBeenCalledWith("all");
    fireEvent.click(screen.getByRole("radio", { name: "Неделя" }));
    expect(useFunnelAnalytics).toHaveBeenLastCalledWith("week");
  });
});
