import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";

import { server, url } from "@/__tests__/integration/server";
import type { InboxFlowResponse } from "@/lib/api";

// AnalyticsTabs uses usePathname and LeadsByStatusTable uses useRouter — both
// require the Next app-router context, which jsdom has no provider for. Mock
// the navigation boundary so the real page/components render.
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/analytics/inbox",
}));

import InboxAnalyticsPage from "./page";

// Integration: real InboxAnalyticsPage + useInboxAnalytics + lib/api.ts,
// network via MSW. The page fires a single fetch on mount:
//   GET /api/analytics/inbox?period=month
function inboxFlow(overrides: Partial<InboxFlowResponse> = {}): InboxFlowResponse {
  return {
    period: { from: "2026-06-01", to: "2026-06-25" },
    leads: {
      total: 12,
      by_channel: { telegram: 8, email: 4 },
      by_status: { new: 5, qualified: 7 },
    },
    qualifications: {
      score_histogram: [
        { range: "0-25", count: 1 },
        { range: "26-50", count: 3 },
        { range: "51-75", count: 5 },
        { range: "76-100", count: 3 },
      ],
      avg_score: 62,
    },
    pending_replies: {
      approved: 9,
      rejected: 3,
      currently_pending: 2,
      approve_rate: 0.75,
      p50_time_to_decide_seconds: 90,
      p95_time_to_decide_seconds: 300,
    },
    ...overrides,
  };
}

function mountWith(period: AnalyticsPeriodLike, body: InboxFlowResponse) {
  server.use(
    http.get(url("/api/analytics/inbox"), ({ request }) => {
      const got = new URL(request.url).searchParams.get("period");
      if (got !== period) return new HttpResponse(null, { status: 500 });
      return HttpResponse.json(body);
    }),
  );
}

type AnalyticsPeriodLike = "week" | "month" | "all";

describe("inbox analytics page (integration)", () => {
  it("loads the inbox flow data from the API and renders it", async () => {
    mountWith("month", inboxFlow());

    render(<InboxAnalyticsPage />);

    // Header is static; the channel counts come from the mocked API.
    expect(await screen.findByText("Лиды по каналам")).toBeInTheDocument();

    // Channel split: telegram 8, email 4 (rendered inside per-channel rows).
    expect(screen.getByTestId("channel-telegram")).toHaveTextContent("8");
    expect(screen.getByTestId("channel-email")).toHaveTextContent("4");

    // Pending-replies (HITL) metrics from the same payload.
    expect(screen.getByTestId("pr-approved")).toHaveTextContent("9");
    expect(screen.getByTestId("pr-rejected")).toHaveTextContent("3");
    expect(screen.getByTestId("pr-pending")).toHaveTextContent("2");
  });

  it("refetches with the selected period and re-renders the new data", async () => {
    const user = userEvent.setup({ delay: null });

    server.use(
      http.get(url("/api/analytics/inbox"), ({ request }) => {
        const period = new URL(request.url).searchParams.get("period");
        if (period === "week") {
          return HttpResponse.json(
            inboxFlow({ leads: { total: 2, by_channel: { telegram: 2, email: 0 }, by_status: {} } }),
          );
        }
        return HttpResponse.json(inboxFlow());
      }),
    );

    render(<InboxAnalyticsPage />);
    await screen.findByText("Лиды по каналам");
    expect(screen.getByTestId("channel-telegram")).toHaveTextContent("8");

    // PeriodSelector is a radiogroup; picking "Неделя" -> hook refetches with
    // ?period=week and the card re-renders the new telegram count (2).
    await user.click(screen.getByRole("radio", { name: "Неделя" }));

    await screen.findByText(
      (_, el) => el?.getAttribute("data-testid") === "channel-telegram" && !!el.textContent?.includes("2"),
    );
    expect(screen.getByTestId("channel-telegram")).toHaveTextContent("2");
  });
});
