import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";

import { server, url } from "@/__tests__/integration/server";
import type { HotLead, HotLeadsResponse } from "@/lib/api";

const pushMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock, back: vi.fn() }),
  usePathname: () => "/analytics/hot-leads",
}));

import HotLeadsAnalyticsPage from "./page";

// Integration: real HotLeadsAnalyticsPage + useHotLeads + lib/api.ts, network via MSW.
// The only request fired on mount is GET /api/analytics/hot-leads.
function hotLead(over: Partial<HotLead> = {}): HotLead {
  return {
    id: "l1",
    contact_name: "Иван Петров",
    channel: "telegram",
    status: "qualified",
    score: 90,
    score_reason: "Бюджет подтверждён",
    last_activity_at: "2026-06-20T10:00:00Z",
    qualified_at: "2026-06-20T09:00:00Z",
    ...over,
  };
}

function mountWith(body: HotLeadsResponse, extra: Parameters<typeof server.use> = []) {
  server.use(
    http.get(url("/api/analytics/hot-leads"), () => HttpResponse.json(body)),
    ...extra,
  );
}

describe("hot-leads analytics page (integration)", () => {
  it("loads data from the API and renders it", async () => {
    mountWith({
      leads: [
        hotLead({ id: "a", contact_name: "Иван Петров", score: 90 }),
        hotLead({ id: "b", contact_name: "Мария Сидорова", channel: "email", status: "new", score: 40 }),
      ],
      total_matching: 2,
      limit_applied: 50,
    });

    render(<HotLeadsAnalyticsPage />);

    expect(await screen.findByText("Иван Петров")).toBeInTheDocument();
    expect(screen.getByText("Мария Сидорова")).toBeInTheDocument();
    // Status label resolved from the raw API code, and the score rendered.
    expect(screen.getByText("Квалифицирован")).toBeInTheDocument();
    expect(screen.getByText("90")).toBeInTheDocument();
    // Footer summary reflects the API total_matching.
    expect(screen.getByText(/Показано 2 из 2/)).toBeInTheDocument();
  });

  it("renders the empty-state when the API returns no leads", async () => {
    mountWith({ leads: [], total_matching: 0, limit_applied: 50 });

    render(<HotLeadsAnalyticsPage />);

    expect(await screen.findByText("Нет лидов под выбранные фильтры.")).toBeInTheDocument();
  });

  it("navigates to the lead's inbox view when a row is clicked", async () => {
    const user = userEvent.setup({ delay: null });
    mountWith({
      leads: [hotLead({ id: "lead-42", contact_name: "Иван Петров" })],
      total_matching: 1,
      limit_applied: 50,
    });

    render(<HotLeadsAnalyticsPage />);
    await user.click(await screen.findByText("Иван Петров"));

    expect(pushMock).toHaveBeenCalledWith("/inbox/lead-42");
  });
});
