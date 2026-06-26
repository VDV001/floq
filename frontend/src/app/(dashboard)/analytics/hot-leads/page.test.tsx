import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { HotLead, HotLeadsParams } from "@/lib/api";

vi.mock("next/navigation", () => ({
  usePathname: () => "/analytics/hot-leads",
  useRouter: () => ({ push: vi.fn() }),
}));

interface HookState {
  leads: HotLead[];
  totalMatching: number;
  loading: boolean;
  error: Error | null;
  lastUpdated: Date | null;
  refresh: ReturnType<typeof vi.fn>;
}

const hookState: HookState = {
  leads: [],
  totalMatching: 0,
  loading: false,
  error: null,
  lastUpdated: null,
  refresh: vi.fn(),
};

const useHotLeads = vi.fn((_params: HotLeadsParams) => hookState);
vi.mock("@/hooks/useHotLeads", () => ({
  useHotLeads: (params: HotLeadsParams) => useHotLeads(params),
}));

import HotLeadsAnalyticsPage from "./page";

const leads: HotLead[] = [
  {
    id: "l1",
    contact_name: "Alice Corp",
    channel: "telegram",
    status: "qualified",
    score: 92,
    score_reason: "hot",
    last_activity_at: "2026-06-24T12:00:00Z",
    qualified_at: "2026-06-24T11:00:00Z",
  },
];

beforeEach(() => {
  hookState.leads = [];
  hookState.totalMatching = 0;
  hookState.loading = false;
  hookState.error = null;
  hookState.lastUpdated = null;
  hookState.refresh = vi.fn();
  useHotLeads.mockClear();
});

describe("HotLeadsAnalyticsPage (unit)", () => {
  it("renders the loading placeholder while loading with no leads yet", () => {
    hookState.loading = true;
    render(<HotLeadsAnalyticsPage />);
    expect(screen.getByText("Загружаем…")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Горячие лиды" })).toBeInTheDocument();
  });

  it("renders an error banner when the hook reports an error", () => {
    hookState.error = new Error("api fail");
    render(<HotLeadsAnalyticsPage />);
    expect(screen.getByRole("alert")).toHaveTextContent("Не удалось загрузить данные: api fail");
  });

  it("renders the table empty-state message when there are no leads and not loading", () => {
    render(<HotLeadsAnalyticsPage />);
    expect(screen.getByText("Нет лидов под выбранные фильтры.")).toBeInTheDocument();
    expect(screen.queryByText("Загружаем…")).not.toBeInTheDocument();
  });

  it("renders rows and the count summary when leads are present", () => {
    hookState.leads = leads;
    hookState.totalMatching = 5;
    hookState.lastUpdated = new Date("2026-06-25T10:00:00Z");
    render(<HotLeadsAnalyticsPage />);
    expect(screen.getByText("Alice Corp")).toBeInTheDocument();
    expect(screen.getByText(/Показано 1 из 5/)).toBeInTheDocument();
  });

  it("calls refresh when the Обновить button is clicked", () => {
    render(<HotLeadsAnalyticsPage />);
    fireEvent.click(screen.getByRole("button", { name: "Обновить" }));
    expect(hookState.refresh).toHaveBeenCalledTimes(1);
  });

  it("re-queries the hook with the new period when PeriodSelector changes", () => {
    render(<HotLeadsAnalyticsPage />);
    expect(useHotLeads).toHaveBeenCalledWith(
      expect.objectContaining({ period: "all", status: "any", channel: "any" }),
    );
    fireEvent.click(screen.getByRole("radio", { name: "Неделя" }));
    expect(useHotLeads).toHaveBeenLastCalledWith(expect.objectContaining({ period: "week" }));
  });

  it("re-queries the hook with the new status filter", () => {
    render(<HotLeadsAnalyticsPage />);
    fireEvent.change(screen.getByLabelText("Статус"), { target: { value: "qualified" } });
    expect(useHotLeads).toHaveBeenLastCalledWith(expect.objectContaining({ status: "qualified" }));
  });
});
