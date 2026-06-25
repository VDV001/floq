import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { http, HttpResponse } from "msw";

import { server, url } from "@/__tests__/integration/server";
import type { Lead } from "@/lib/api";

import PipelinePage from "./page";

// Integration: real PipelinePage + usePipelinePage + lib/api.ts, network via MSW.
// On mount the page fires:
//   GET /api/leads                          (kanban columns + metric cards)
//   GET /api/leads/:id/qualification        (one per lead, for the card preview)
function lead(overrides: Partial<Lead> = {}): Lead {
  return {
    id: "l1",
    user_id: "u1",
    channel: "email",
    contact_name: "Иван Петров",
    company: "Acme",
    first_message: "Здравствуйте",
    status: "new",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    ...overrides,
  };
}

function mountWith(leads: Lead[], extra: Parameters<typeof server.use> = []) {
  server.use(
    http.get(url("/api/leads"), () => HttpResponse.json(leads)),
    // Each card asks for its qualification; respond 404 so the preview stays empty.
    http.get(url("/api/leads/:id/qualification"), () =>
      HttpResponse.json({ error: "not found" }, { status: 404 }),
    ),
    ...extra,
  );
}

describe("pipeline page (integration)", () => {
  it("loads leads from the API and renders them across kanban columns", async () => {
    mountWith([
      lead({ id: "a", contact_name: "Иван Петров", company: "Acme", status: "new", channel: "email" }),
      lead({ id: "b", contact_name: "Мария Сидорова", company: "Globex", status: "qualified", channel: "telegram" }),
    ]);

    render(<PipelinePage />);

    expect(await screen.findByText("Иван Петров")).toBeInTheDocument();
    expect(screen.getByText("Мария Сидорова")).toBeInTheDocument();
    // Column headers are always rendered.
    expect(screen.getByText("Новый")).toBeInTheDocument();
    expect(screen.getByText("Квалифицирован")).toBeInTheDocument();
    // "Всего активных" metric counts non-closed leads (both here).
    expect(screen.getByText("2")).toBeInTheDocument();
  });

  it("filters cards by the selected channel", async () => {
    mountWith([
      lead({ id: "a", contact_name: "Иван Петров", channel: "email" }),
      lead({ id: "b", contact_name: "Мария Сидорова", channel: "telegram", status: "qualified" }),
    ]);

    render(<PipelinePage />);
    await screen.findByText("Иван Петров");

    // Switching to the Telegram channel hides the email lead.
    fireEvent.click(screen.getByRole("button", { name: "Telegram" }));

    expect(screen.queryByText("Иван Петров")).not.toBeInTheDocument();
    expect(screen.getByText("Мария Сидорова")).toBeInTheDocument();
  });
});
