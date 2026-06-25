import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { http, HttpResponse } from "msw";

import { server, url } from "@/__tests__/integration/server";
import { NotificationProvider } from "@/components/notifications/NotificationProvider";
import type { Lead } from "@/lib/api";

// LeadCard + PendingQueueTabs use next/link + usePathname; pin the router
// so the page mounts deterministically without a Next router context.
vi.mock("next/navigation", () => ({
  usePathname: () => "/inbox",
}));

import InboxPage from "./page";

// Minimal valid Lead for inbox fixtures; override per test. Defaults to the
// "new" status so the row lands in the default pipeline stage.
function lead(over: Partial<Lead> = {}): Lead {
  return {
    id: over.id ?? "l-1",
    user_id: over.user_id ?? "u-1",
    channel: over.channel ?? "telegram",
    contact_name: over.contact_name ?? "Иван Петров",
    company: over.company ?? "Acme",
    first_message: over.first_message ?? "Здравствуйте, интересует демо",
    status: over.status ?? "new",
    source_name: over.source_name ?? "Источник A",
    created_at: over.created_at ?? "2026-06-25T10:00:00Z",
    updated_at: over.updated_at ?? "2026-06-25T10:00:00Z",
    pending_replies_count: over.pending_replies_count ?? 0,
  };
}

// Integration: real InboxPage + useInboxPage + lib/api.ts, network via MSW.
// On mount the page fires two GETs:
//   /api/leads                    (lead feed -> sidebar counts + cards)
//   /api/leads/suggestion-counts  (cross-channel dedup badges)
function mountWith(leads: Lead[], extra: Parameters<typeof server.use> = []) {
  server.use(
    http.get(url("/api/leads"), () => HttpResponse.json(leads)),
    http.get(url("/api/leads/suggestion-counts"), () => HttpResponse.json({})),
    ...extra,
  );
}

describe("inbox page (integration)", () => {
  it("loads leads from the API and renders the list", async () => {
    mountWith([
      lead({ id: "a", company: "Acme", contact_name: "Иван Петров" }),
      lead({ id: "b", company: "Globex", contact_name: "Мария Сидорова" }),
    ]);

    render(
      <NotificationProvider>
        <InboxPage />
      </NotificationProvider>,
    );

    // Cards rendered from the API response (default "new" stage).
    expect(await screen.findByText("Acme")).toBeInTheDocument();
    expect(screen.getByText("Globex")).toBeInTheDocument();
    // The header count reflects the loaded leads.
    expect(screen.getByText(/Показано 2 активных лидов/)).toBeInTheDocument();
  });

  it("filters the rendered cards by the selected pipeline stage", async () => {
    mountWith([
      lead({ id: "a", company: "Acme", status: "new" }),
      lead({ id: "b", company: "Globex", status: "qualified" }),
    ]);

    render(
      <NotificationProvider>
        <InboxPage />
      </NotificationProvider>,
    );
    await screen.findByText("Acme");

    // Default stage "new" hides the qualified lead.
    expect(screen.queryByText("Globex")).not.toBeInTheDocument();

    // Switch to the "Квалифицированные" stage -> qualified lead shows,
    // the new lead is filtered out.
    fireEvent.click(screen.getByText("Квалифицированные"));

    expect(await screen.findByText("Globex")).toBeInTheDocument();
    expect(screen.queryByText("Acme")).not.toBeInTheDocument();
  });

  it("filters the rendered cards by the selected source", async () => {
    mountWith([
      lead({ id: "a", company: "Acme", source_name: "Источник A" }),
      lead({ id: "b", company: "Globex", source_name: "Источник B" }),
    ]);

    render(
      <NotificationProvider>
        <InboxPage />
      </NotificationProvider>,
    );
    await screen.findByText("Acme");
    expect(screen.getByText("Globex")).toBeInTheDocument();

    // The sidebar source <select> drives the sourceFilter -> only B remains.
    fireEvent.change(screen.getByRole("combobox"), {
      target: { value: "Источник B" },
    });

    expect(await screen.findByText("Globex")).toBeInTheDocument();
    expect(screen.queryByText("Acme")).not.toBeInTheDocument();
  });

  it("resolves a /start lead preview from its qualification", async () => {
    mountWith(
      [lead({ id: "a", company: "Acme", first_message: "/start" })],
      [
        http.get(url("/api/leads/a/qualification"), () =>
          HttpResponse.json({ identified_need: "Нужна CRM-интеграция" }),
        ),
      ],
    );

    render(
      <NotificationProvider>
        <InboxPage />
      </NotificationProvider>,
    );

    // "/start" first message renders a placeholder, then the qualification
    // fetch backfills the identified need into the card preview.
    expect(await screen.findByText("Нужна CRM-интеграция")).toBeInTheDocument();
  });

  it("renders the empty state when the leads request fails", async () => {
    server.use(
      http.get(url("/api/leads"), () => new HttpResponse(null, { status: 500 })),
      http.get(url("/api/leads/suggestion-counts"), () => HttpResponse.json({})),
    );

    render(
      <NotificationProvider>
        <InboxPage />
      </NotificationProvider>,
    );

    // The failed fetch is swallowed; loading clears and the empty state shows.
    expect(
      await screen.findByText("Пока нет входящих лидов"),
    ).toBeInTheDocument();
  });
});
