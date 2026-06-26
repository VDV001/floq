import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";

import { server, url } from "@/__tests__/integration/server";
import { NotificationProvider } from "@/components/notifications/NotificationProvider";
import type { Lead, Message, Qualification, Draft } from "@/lib/api";

// Dynamic route [leadId]: the page reads the id via useParams(). Pin it to
// "lead-1" and stub usePathname (used by child <Link>/router context) so the
// page mounts deterministically without a real Next router.
const { pushMock } = vi.hoisted(() => ({ pushMock: vi.fn() }));
vi.mock("next/navigation", () => ({
  useParams: () => ({ leadId: "lead-1" }),
  usePathname: () => "/inbox/lead-1",
  useRouter: () => ({ push: pushMock, replace: vi.fn(), back: vi.fn() }),
}));

import LeadDetailPage from "./page";

function lead(over: Partial<Lead> = {}): Lead {
  return {
    id: over.id ?? "lead-1",
    user_id: over.user_id ?? "u-1",
    channel: over.channel ?? "telegram",
    contact_name: over.contact_name ?? "Иван Петров",
    company: over.company ?? "Acme",
    first_message: over.first_message ?? "Здравствуйте, интересует демо",
    status: over.status ?? "new",
    email_address: over.email_address,
    source_name: over.source_name ?? "Источник A",
    created_at: over.created_at ?? "2026-06-25T10:00:00Z",
    updated_at: over.updated_at ?? "2026-06-25T10:00:00Z",
    pending_replies_count: over.pending_replies_count ?? 0,
    archived_at: over.archived_at,
  };
}

function message(over: Partial<Message> = {}): Message {
  return {
    id: over.id ?? "m-1",
    lead_id: over.lead_id ?? "lead-1",
    direction: over.direction ?? "inbound",
    body: over.body ?? "Здравствуйте, интересует демо",
    sent_at: over.sent_at ?? "2026-06-25T10:00:00Z",
  };
}

function qualification(over: Partial<Qualification> = {}): Qualification {
  return {
    id: over.id ?? "q-1",
    lead_id: over.lead_id ?? "lead-1",
    identified_need: over.identified_need ?? "CRM-интеграция",
    estimated_budget: over.estimated_budget ?? "500k",
    deadline: over.deadline ?? "Q3",
    score: over.score ?? 80,
    score_reason: over.score_reason ?? "горячий лид",
    recommended_action: over.recommended_action ?? "позвонить",
    provider_used: over.provider_used ?? "openai",
    generated_at: over.generated_at ?? "2026-06-25T10:00:00Z",
  };
}

function draft(over: Partial<Draft> = {}): Draft {
  return {
    id: over.id ?? "d-1",
    lead_id: over.lead_id ?? "lead-1",
    body: over.body ?? "Предлагаю созвон в четверг",
    created_at: over.created_at ?? "2026-06-25T10:00:00Z",
  };
}

// Integration: real LeadDetailPage + real child components + lib/api.ts,
// network via MSW. On mount the page (and its children) fire many GETs —
// all are stubbed here. server.listen({ onUnhandledRequest: "error" }) means
// any unmocked request fails the test.
//   GET /api/settings                          -> inbox-view preference
//   GET /api/leads/lead-1                       -> lead header/contact info
//   GET /api/leads/lead-1/messages              -> conversation thread
//   GET /api/leads/lead-1/qualification         -> qualification card
//   GET /api/leads/lead-1/draft                 -> draft sidebar
//   GET /api/leads/lead-1/prospect-suggestions  -> ProspectSuggestionBanner
//   GET /api/leads/lead-1/pending-replies       -> PendingReplySection
// MSW resolves the FIRST matching handler registered via server.use, so we
// take a single messages responder rather than a static body + override.
function mountWith(opts: {
  lead?: Lead;
  messagesResponder?: Parameters<typeof http.get>[1];
  qualification?: Qualification | null;
  draft?: Draft | null;
  extra?: Parameters<typeof server.use>;
}) {
  server.use(
    http.get(url("/api/settings"), () =>
      HttpResponse.json({ aggregated_inbox_view: true }),
    ),
    http.get(url("/api/leads/lead-1"), () => HttpResponse.json(opts.lead ?? lead())),
    // url() ignores the query string, so this single handler covers both the
    // aggregated mount fetch (?aggregated=true) and the plain post-send refetch.
    http.get(
      url("/api/leads/lead-1/messages"),
      opts.messagesResponder ?? (() => HttpResponse.json([message()])),
    ),
    http.get(url("/api/leads/lead-1/qualification"), () =>
      opts.qualification === null
        ? new HttpResponse(null, { status: 404 })
        : HttpResponse.json(opts.qualification ?? qualification()),
    ),
    http.get(url("/api/leads/lead-1/draft"), () =>
      opts.draft === null
        ? new HttpResponse(null, { status: 404 })
        : HttpResponse.json(opts.draft ?? draft()),
    ),
    http.get(url("/api/leads/lead-1/prospect-suggestions"), () =>
      HttpResponse.json([]),
    ),
    http.get(url("/api/leads/lead-1/pending-replies"), () => HttpResponse.json([])),
    ...(opts.extra ?? []),
  );
}

function renderPage() {
  render(
    <NotificationProvider>
      <LeadDetailPage />
    </NotificationProvider>,
  );
}

describe("lead detail page (integration)", () => {
  it("loads the lead conversation from the API and renders messages + contact info", async () => {
    mountWith({
      lead: lead({ contact_name: "Мария Сидорова", company: "Globex" }),
      messagesResponder: () =>
        HttpResponse.json([
          message({ id: "m-1", direction: "inbound", body: "Нужна интеграция с 1С" }),
          message({ id: "m-2", direction: "outbound", body: "Поможем — расскажите подробнее" }),
        ]),
    });

    renderPage();

    // Contact info from GET /api/leads/lead-1.
    expect(await screen.findByText("Мария Сидорова")).toBeInTheDocument();
    expect(screen.getByText("Globex")).toBeInTheDocument();

    // Conversation thread from GET /api/leads/lead-1/messages.
    expect(screen.getByText("Нужна интеграция с 1С")).toBeInTheDocument();
    expect(screen.getByText("Поможем — расскажите подробнее")).toBeInTheDocument();
  });

  it("renders the company enrichment card for an email lead", async () => {
    mountWith({
      lead: lead({ channel: "email", email_address: "ivan@acme.ru" }),
      extra: [
        http.get(url("/api/enrichment"), () =>
          HttpResponse.json({
            domain: "acme.ru",
            status: "enriched",
            profile: { title: "Acme LLC", description: "Делаем виджеты", emails: [], phones: [], socials: [] },
          }),
        ),
      ],
    });

    renderPage();

    expect(await screen.findByText("О компании")).toBeInTheDocument();
    expect(await screen.findByText("Acme LLC")).toBeInTheDocument();
    expect(screen.getByText("Делаем виджеты")).toBeInTheDocument();
  });

  it("sends a manually typed reply through the API and refetches messages", async () => {
    const user = userEvent.setup({ delay: null });
    let sentBody: string | undefined;
    let messagesFetches = 0;

    mountWith({
      lead: lead(),
      draft: null, // empty textarea so we type the reply ourselves
      // Start with one inbound message; after send the refetch returns two.
      messagesResponder: () => {
        messagesFetches += 1;
        return messagesFetches === 1
          ? HttpResponse.json([message({ id: "m-1", body: "Здравствуйте" })])
          : HttpResponse.json([
              message({ id: "m-1", body: "Здравствуйте" }),
              message({ id: "m-2", direction: "outbound", body: "Спасибо за обращение!" }),
            ]);
      },
      extra: [
        http.post(url("/api/leads/lead-1/send"), async ({ request }) => {
          sentBody = ((await request.json()) as { body: string }).body;
          return HttpResponse.json(
            message({ id: "m-2", direction: "outbound", body: "Спасибо за обращение!" }),
          );
        }),
      ],
    });

    renderPage();

    // Wait for the page to finish loading (draft sidebar textarea is present).
    const textarea = await screen.findByPlaceholderText(
      "Напишите ответ вручную или сгенерируйте черновик кнопкой ниже.",
    );

    fireEvent.change(textarea, { target: { value: "Спасибо за обращение!" } });
    await user.click(screen.getByRole("button", { name: /Отправить ответ/ }));

    // POST body reached the API.
    await waitFor(() => expect(sentBody).toBe("Спасибо за обращение!"));

    // The outbound message from the post-send refetch is now in the thread.
    expect(await screen.findByText("Спасибо за обращение!")).toBeInTheDocument();
    // At least one extra messages fetch happened after the initial mount load.
    expect(messagesFetches).toBeGreaterThanOrEqual(2);
  });

  it("archives the lead via the API and redirects to the inbox feed", async () => {
    const user = userEvent.setup({ delay: null });
    pushMock.mockClear();
    let archiveHit = false;

    mountWith({
      lead: lead({ contact_name: "Архив Тест" }),
      extra: [
        http.post(url("/api/leads/lead-1/archive"), () => {
          archiveHit = true;
          return HttpResponse.json({ status: "archived" });
        }),
      ],
    });

    renderPage();

    // Archive is a two-step confirm so it can't fire accidentally.
    await user.click(await screen.findByRole("button", { name: "Архив" }));
    await user.click(await screen.findByRole("button", { name: /Да, в архив/ }));

    await waitFor(() => expect(archiveHit).toBe(true));
    // On success the lead leaves the working feed → redirect to /inbox.
    await waitFor(() => expect(pushMock).toHaveBeenCalledWith("/inbox"));
  });

  it("cancels the archive confirm without calling the API", async () => {
    const user = userEvent.setup({ delay: null });
    pushMock.mockClear();
    let archiveHit = false;

    mountWith({
      lead: lead({ contact_name: "Архив Отмена" }),
      extra: [
        http.post(url("/api/leads/lead-1/archive"), () => {
          archiveHit = true;
          return HttpResponse.json({ status: "archived" });
        }),
      ],
    });

    renderPage();

    await user.click(await screen.findByRole("button", { name: "Архив" }));
    await user.click(await screen.findByRole("button", { name: /Отмена/ }));

    // Back to the idle Архив button; the API was never hit.
    expect(await screen.findByRole("button", { name: "Архив" })).toBeInTheDocument();
    expect(archiveHit).toBe(false);
    expect(pushMock).not.toHaveBeenCalled();
  });

  it("shows Разархивировать (not Архив) when the lead is archived", async () => {
    mountWith({
      lead: lead({ contact_name: "Архивный", archived_at: "2026-06-25T11:00:00Z" }),
    });

    renderPage();

    expect(await screen.findByRole("button", { name: /Разархивировать/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Архив" })).not.toBeInTheDocument();
  });

  it("unarchives an archived lead via the API and swaps back to the Архив affordance", async () => {
    const user = userEvent.setup({ delay: null });
    let unarchiveHit = false;

    mountWith({
      lead: lead({ contact_name: "Вернуть", archived_at: "2026-06-25T11:00:00Z" }),
      extra: [
        http.post(url("/api/leads/lead-1/unarchive"), () => {
          unarchiveHit = true;
          return HttpResponse.json({ status: "active" });
        }),
      ],
    });

    renderPage();

    await user.click(await screen.findByRole("button", { name: /Разархивировать/ }));

    await waitFor(() => expect(unarchiveHit).toBe(true));
    // The lead is active again → the button flips back to the archive control.
    expect(await screen.findByRole("button", { name: "Архив" })).toBeInTheDocument();
    expect(screen.getByText("Лид возвращён")).toBeInTheDocument();
  });

  it("surfaces an error and stays on the page when archive fails", async () => {
    const user = userEvent.setup({ delay: null });
    pushMock.mockClear();

    mountWith({
      lead: lead({ contact_name: "Архив Ошибка" }),
      extra: [
        http.post(url("/api/leads/lead-1/archive"), () =>
          HttpResponse.json({ error: "boom" }, { status: 500 }),
        ),
      ],
    });

    renderPage();

    await user.click(await screen.findByRole("button", { name: "Архив" }));
    await user.click(await screen.findByRole("button", { name: /Да, в архив/ }));

    // Error notification shown; no redirect.
    expect(await screen.findByText("Не удалось архивировать лид")).toBeInTheDocument();
    expect(pushMock).not.toHaveBeenCalled();
  });
});
