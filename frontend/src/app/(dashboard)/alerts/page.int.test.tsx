import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";

import { server, url } from "@/__tests__/integration/server";
import type { Lead } from "@/lib/api";

import AlertsPage from "./page";

// Integration: real AlertsPage + useAlerts + lib/api.ts, network via MSW.
// The page's only mount fetch is GET /api/leads (useAlerts → api.getLeads()).
function lead(overrides: Partial<Lead> = {}): Lead {
  const now = new Date().toISOString();
  return {
    id: "l1",
    user_id: "u1",
    channel: "email",
    contact_name: "Иван Петров",
    company: "Acme",
    first_message: "Здравствуйте, интересует ваш продукт",
    status: "followup",
    created_at: now,
    updated_at: now,
    ...overrides,
  };
}

function mountWith(leads: Lead[]) {
  server.use(http.get(url("/api/leads"), () => HttpResponse.json(leads)));
}

describe("alerts page (integration)", () => {
  it("loads leads from the API and renders the followup ones", async () => {
    mountWith([
      lead({ id: "a", contact_name: "Иван Петров", company: "Acme", status: "followup" }),
      lead({ id: "b", contact_name: "Мария Сидорова", company: "Globex", status: "followup" }),
      // A non-followup lead must be filtered out of the alerts view.
      lead({ id: "c", contact_name: "Пётр Чужой", status: "new" }),
    ]);

    render(<AlertsPage />);

    // featured = first followup lead (FeaturedCard).
    expect(await screen.findByText("Иван Петров")).toBeInTheDocument();
    // remaining followup leads render as AlertListItem rows.
    expect(screen.getByText("Мария Сидорова")).toBeInTheDocument();
    // the non-followup lead is not shown anywhere.
    expect(screen.queryByText("Пётр Чужой")).not.toBeInTheDocument();
  });

  it("shows the followup count in the header and total leads in the footer", async () => {
    mountWith([
      lead({ id: "a", contact_name: "Иван Петров", status: "followup" }),
      lead({ id: "b", contact_name: "Мария Сидорова", status: "followup" }),
      lead({ id: "c", contact_name: "Пётр Чужой", status: "new" }),
    ]);

    render(<AlertsPage />);
    await screen.findByText("Иван Петров");

    // Header: "{followupLeads.length} лидов" split across spans → match flexibly.
    expect(screen.getByText("2 лидов")).toBeInTheDocument();
    // Footer "Всего лидов" value = total leads from the API (3).
    expect(screen.getByText("3")).toBeInTheDocument();
  });

  it("renders the empty state when no leads need a followup", async () => {
    mountWith([lead({ id: "c", contact_name: "Пётр Чужой", status: "new" })]);

    render(<AlertsPage />);

    expect(await screen.findByText("Нет напоминаний")).toBeInTheDocument();
    expect(
      screen.getByText("Все лиды в работе, фоллоуапов нет."),
    ).toBeInTheDocument();
  });
});
