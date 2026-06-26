import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { InboxFlowResponse } from "@/lib/api";

vi.mock("next/navigation", () => ({
  usePathname: () => "/analytics/inbox",
  useRouter: () => ({ push: vi.fn() }),
}));

interface HookState {
  data: InboxFlowResponse | null;
  loading: boolean;
  error: Error | null;
  lastUpdated: Date | null;
  refresh: ReturnType<typeof vi.fn>;
}

const hookState: HookState = {
  data: null,
  loading: false,
  error: null,
  lastUpdated: null,
  refresh: vi.fn(),
};

const useInboxAnalytics = vi.fn((_period: string) => hookState);
vi.mock("@/hooks/useInboxAnalytics", () => ({
  useInboxAnalytics: (period: string) => useInboxAnalytics(period),
}));

import InboxAnalyticsPage from "./page";

const data: InboxFlowResponse = {
  period: { from: "2026-05-26", to: "2026-06-25" },
  leads: {
    total: 30,
    by_channel: { telegram: 18, email: 12 },
    by_status: { new: 10, qualified: 20 },
  },
  qualifications: {
    score_histogram: [{ range: "80-100", count: 5 }],
    avg_score: 71.5,
  },
  pending_replies: {
    approved: 8,
    rejected: 2,
    currently_pending: 3,
    approve_rate: 0.8,
    p50_time_to_decide_seconds: 120,
    p95_time_to_decide_seconds: 600,
  },
};

beforeEach(() => {
  hookState.data = null;
  hookState.loading = false;
  hookState.error = null;
  hookState.lastUpdated = null;
  hookState.refresh = vi.fn();
  useInboxAnalytics.mockClear();
});

describe("InboxAnalyticsPage (unit)", () => {
  it("renders the loading placeholder while loading with no data yet", () => {
    hookState.loading = true;
    render(<InboxAnalyticsPage />);
    expect(screen.getByText("Загружаем…")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Входящие" })).toBeInTheDocument();
  });

  it("renders an error banner when the hook reports an error", () => {
    hookState.error = new Error("inbox boom");
    render(<InboxAnalyticsPage />);
    expect(screen.getByRole("alert")).toHaveTextContent("Не удалось загрузить данные: inbox boom");
  });

  it("renders no cards in the empty state (no data, not loading)", () => {
    render(<InboxAnalyticsPage />);
    expect(screen.queryByText("Загружаем…")).not.toBeInTheDocument();
    expect(screen.queryByText("Лиды по каналам")).not.toBeInTheDocument();
  });

  it("renders the channel/status/qualification cards when data is present", () => {
    hookState.data = data;
    hookState.lastUpdated = new Date("2026-06-25T10:00:00Z");
    render(<InboxAnalyticsPage />);
    expect(screen.getByText("Лиды по каналам")).toBeInTheDocument();
    expect(screen.getByTestId("channel-telegram")).toBeInTheDocument();
    expect(screen.getByText(/Обновлено/)).toBeInTheDocument();
  });

  it("calls refresh when the Обновить button is clicked", () => {
    render(<InboxAnalyticsPage />);
    fireEvent.click(screen.getByRole("button", { name: "Обновить" }));
    expect(hookState.refresh).toHaveBeenCalledTimes(1);
  });

  it("re-queries the hook with the new period when PeriodSelector changes", () => {
    render(<InboxAnalyticsPage />);
    expect(useInboxAnalytics).toHaveBeenCalledWith("month");
    fireEvent.click(screen.getByRole("radio", { name: "Всё время" }));
    expect(useInboxAnalytics).toHaveBeenLastCalledWith("all");
  });
});
