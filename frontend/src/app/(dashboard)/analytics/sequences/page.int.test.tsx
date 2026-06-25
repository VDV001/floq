import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";

import { server, url } from "@/__tests__/integration/server";
import type { SequenceAnalyticsResponse, SequenceAnalyticsRow } from "@/lib/api";

// AnalyticsTabs calls usePathname(); pin it so the page mounts deterministically
// without a Next router context.
vi.mock("next/navigation", () => ({
  usePathname: () => "/analytics/sequences",
}));

import SequenceAnalyticsPage from "./page";

// Integration: real SequenceAnalyticsPage + useSequenceAnalytics + lib/api.ts,
// network via MSW. On mount the page fires exactly one GET:
//   /api/analytics/sequences?period=all
function row(overrides: Partial<SequenceAnalyticsRow> = {}): SequenceAnalyticsRow {
  return {
    id: "s1",
    name: "Cold outreach",
    sent: 100,
    delivered: 95,
    opened: 50,
    replied: 10,
    converted: 4,
    open_rate: 0.5,
    reply_rate: 0.1,
    conversion_rate: 0.04,
    ...overrides,
  };
}

function mountWith(sequences: SequenceAnalyticsRow[], extra: Parameters<typeof server.use> = []) {
  const response: SequenceAnalyticsResponse = { sequences, period: "all" };
  server.use(
    http.get(url("/api/analytics/sequences"), () => HttpResponse.json(response)),
    ...extra,
  );
}

describe("sequence analytics page (integration)", () => {
  it("loads data from the API and renders the per-sequence rows", async () => {
    mountWith([
      row({ id: "s1", name: "Cold outreach", sent: 100, replied: 10, reply_rate: 0.1 }),
      row({ id: "s2", name: "Warm follow-up", sent: 40, replied: 8, reply_rate: 0.2 }),
    ]);

    render(<SequenceAnalyticsPage />);

    // Sequence names from the API response.
    expect(await screen.findByText("Cold outreach")).toBeInTheDocument();
    expect(screen.getByText("Warm follow-up")).toBeInTheDocument();
    // A numeric metric cell (sent count) and a formatted percentage.
    expect(screen.getByText("100")).toBeInTheDocument();
    expect(screen.getByText("20.0%")).toBeInTheDocument();
  });

  it("re-sorts the table when a column header is clicked", async () => {
    const user = userEvent.setup({ delay: null });
    mountWith([
      row({ id: "s1", name: "Cold outreach", sent: 100 }),
      row({ id: "s2", name: "Warm follow-up", sent: 40 }),
    ]);

    render(<SequenceAnalyticsPage />);
    await screen.findByText("Cold outreach");

    // Default sort is by sent descending: Cold outreach (100) before Warm follow-up (40).
    const namesBefore = screen.getAllByText(/Cold outreach|Warm follow-up/);
    expect(namesBefore[0]).toHaveTextContent("Cold outreach");

    // Clicking a new column header sorts it descending: by name desc, "Warm
    // follow-up" (W) sorts before "Cold outreach" (C).
    await user.click(screen.getByRole("button", { name: /Sequence/ }));

    const namesAfter = screen.getAllByText(/Cold outreach|Warm follow-up/);
    expect(namesAfter[0]).toHaveTextContent("Warm follow-up");
  });

  it("shows an empty state when the API returns no sequences", async () => {
    mountWith([]);

    render(<SequenceAnalyticsPage />);

    expect(
      await screen.findByText("Нет sequence'ов с активностью в выбранном периоде."),
    ).toBeInTheDocument();
  });
});
