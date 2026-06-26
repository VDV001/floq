import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { SequenceAnalyticsRow } from "@/lib/api";

vi.mock("next/navigation", () => ({
  usePathname: () => "/analytics/sequences",
  useRouter: () => ({ push: vi.fn() }),
}));

interface HookState {
  rows: SequenceAnalyticsRow[];
  loading: boolean;
  error: Error | null;
  lastUpdated: Date | null;
  refresh: ReturnType<typeof vi.fn>;
}

const hookState: HookState = {
  rows: [],
  loading: false,
  error: null,
  lastUpdated: null,
  refresh: vi.fn(),
};

const useSequenceAnalytics = vi.fn((_period: string) => hookState);
vi.mock("@/hooks/useSequenceAnalytics", () => ({
  useSequenceAnalytics: (period: string) => useSequenceAnalytics(period),
}));

import SequenceAnalyticsPage from "./page";

const rows: SequenceAnalyticsRow[] = [
  {
    id: "s1",
    name: "Cold Outreach",
    sent: 100,
    delivered: 95,
    opened: 60,
    replied: 20,
    converted: 5,
    open_rate: 0.6,
    reply_rate: 0.2,
    conversion_rate: 0.05,
  },
];

beforeEach(() => {
  hookState.rows = [];
  hookState.loading = false;
  hookState.error = null;
  hookState.lastUpdated = null;
  hookState.refresh = vi.fn();
  useSequenceAnalytics.mockClear();
});

describe("SequenceAnalyticsPage (unit)", () => {
  it("renders the loading placeholder while loading with no rows yet", () => {
    hookState.loading = true;
    render(<SequenceAnalyticsPage />);
    expect(screen.getByText("Загружаем…")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /Аналитика sequence/ })).toBeInTheDocument();
  });

  it("renders an error banner when the hook reports an error", () => {
    hookState.error = new Error("seq fail");
    render(<SequenceAnalyticsPage />);
    expect(screen.getByRole("alert")).toHaveTextContent("Не удалось загрузить данные: seq fail");
  });

  it("renders the table empty-state message when there are no rows and not loading", () => {
    render(<SequenceAnalyticsPage />);
    expect(screen.getByText(/Нет sequence.*с активностью в выбранном периоде/)).toBeInTheDocument();
    expect(screen.queryByText("Загружаем…")).not.toBeInTheDocument();
  });

  it("renders the stats table when rows are present", () => {
    hookState.rows = rows;
    hookState.lastUpdated = new Date("2026-06-25T10:00:00Z");
    render(<SequenceAnalyticsPage />);
    expect(screen.getByText("Cold Outreach")).toBeInTheDocument();
    expect(screen.getByText(/Обновлено:/)).toBeInTheDocument();
  });

  it("calls refresh when the Обновить button is clicked", () => {
    render(<SequenceAnalyticsPage />);
    fireEvent.click(screen.getByRole("button", { name: "Обновить" }));
    expect(hookState.refresh).toHaveBeenCalledTimes(1);
  });

  it("re-queries the hook with the new period when PeriodSelector changes", () => {
    render(<SequenceAnalyticsPage />);
    expect(useSequenceAnalytics).toHaveBeenCalledWith("all");
    fireEvent.click(screen.getByRole("radio", { name: "Месяц" }));
    expect(useSequenceAnalytics).toHaveBeenLastCalledWith("month");
  });
});
