import { describe, it, expect } from "vitest";
import { render, screen, fireEvent, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";

import { server, url } from "@/__tests__/integration/server";
import type { OutboundMessage, OutboundStats } from "@/lib/api";

import OutboundPage from "./page";

// Minimal valid OutboundMessage; override per test. Defaults to a "draft"
// email so it lands in the default "queue" tab.
function outbound(over: Partial<OutboundMessage> = {}): OutboundMessage {
  return {
    id: over.id ?? "m-1",
    prospect_id: over.prospect_id ?? "prospect-abc123",
    sequence_id: over.sequence_id ?? "seq-00112233",
    step_order: over.step_order ?? 1,
    channel: over.channel ?? "email",
    body: over.body ?? "Здравствуйте, предлагаем демо нашего продукта",
    status: over.status ?? "draft",
    scheduled_at: over.scheduled_at ?? "2026-06-25T10:00:00Z",
    sent_at: over.sent_at ?? null,
    created_at: over.created_at ?? "2026-06-25T09:00:00Z",
  };
}

const emptyStats: OutboundStats = {
  draft: 0,
  approved: 0,
  sent: 0,
  opened: 0,
  replied: 0,
  bounced: 0,
};

// Integration: real OutboundPage + useOutbound + lib/api.ts, network via MSW.
// On mount the hook fires three GETs:
//   /api/outbound/queue  (queue tab cards)
//   /api/outbound/sent   (sent tab cards)
//   /api/outbound/stats  (stat cards)
function mountWith(
  queue: OutboundMessage[],
  sent: OutboundMessage[] = [],
  stats: OutboundStats = emptyStats,
  extra: Parameters<typeof server.use> = [],
) {
  server.use(
    http.get(url("/api/outbound/queue"), () => HttpResponse.json(queue)),
    http.get(url("/api/outbound/sent"), () => HttpResponse.json(sent)),
    http.get(url("/api/outbound/stats"), () => HttpResponse.json(stats)),
    ...extra,
  );
}

describe("outbound page (integration)", () => {
  it("loads the outbound queue from the API and renders it", async () => {
    mountWith(
      [
        outbound({ id: "a", prospect_id: "alpha000000", body: "Первое письмо проспекту" }),
        outbound({ id: "b", prospect_id: "beta0000000", body: "Второе письмо проспекту" }),
      ],
      [],
      { ...emptyStats, draft: 2 },
    );

    render(<OutboundPage />);

    // Cards rendered from the queue response (prospect label = "Проспект " + id[:6]).
    expect(await screen.findByText("Проспект alpha0")).toBeInTheDocument();
    expect(screen.getByText("Проспект beta00")).toBeInTheDocument();
    // The queue tab shows the count.
    expect(screen.getByText(/В очереди \(2\)/)).toBeInTheDocument();
    // Stat card reflects the loaded stats.
    expect(screen.getByText("2")).toBeInTheDocument();
  });

  it("approves a queued message through the API and removes it from the list", async () => {
    const user = userEvent.setup({ delay: null });
    let approvedId: string | null = null;
    mountWith(
      [
        outbound({ id: "a", prospect_id: "alpha000000" }),
        outbound({ id: "b", prospect_id: "beta0000000" }),
      ],
      [],
      { ...emptyStats, draft: 2 },
      [
        http.post(url("/api/outbound/a/approve"), () => {
          approvedId = "a";
          return new HttpResponse(null, { status: 204 });
        }),
      ],
    );

    render(<OutboundPage />);
    await screen.findByText("Проспект alpha0");

    // Approve the first card.
    const approveButtons = screen.getAllByRole("button", { name: /Подтвердить$/ });
    await user.click(approveButtons[0]);

    // The approved row is removed; the other remains.
    expect(await screen.findByText("Проспект beta00")).toBeInTheDocument();
    expect(screen.queryByText("Проспект alpha0")).not.toBeInTheDocument();
    expect(approvedId).toBe("a");
  });

  it("filters the queue cards by the selected channel", async () => {
    mountWith([
      outbound({ id: "a", prospect_id: "alpha000000", channel: "email" }),
      outbound({ id: "b", prospect_id: "beta0000000", channel: "telegram" }),
    ]);

    render(<OutboundPage />);
    await screen.findByText("Проспект alpha0");

    // Switch the channel filter to Telegram -> only the telegram card remains.
    // (Use the filter button, not the card's channel badge span.)
    fireEvent.click(screen.getByRole("button", { name: "Telegram" }));

    expect(await screen.findByText("Проспект beta00")).toBeInTheDocument();
    expect(screen.queryByText("Проспект alpha0")).not.toBeInTheDocument();
  });

  it("edits a draft body and persists it through the edit endpoint", async () => {
    const user = userEvent.setup({ delay: null });
    let editBody: { body?: string } | null = null;
    mountWith(
      [outbound({ id: "e1", prospect_id: "alpha000000", body: "Старый текст" })],
      [],
      emptyStats,
      [
        http.post(url("/api/outbound/e1/edit"), async ({ request }) => {
          editBody = (await request.json()) as { body: string };
          return new HttpResponse(null, { status: 204 });
        }),
      ],
    );

    render(<OutboundPage />);
    await screen.findByText(/Старый текст/);

    // The pencil is the sibling button right after the card's approve button.
    const approve = screen.getByRole("button", { name: /Подтвердить$/ });
    const cardActions = approve.parentElement as HTMLElement;
    const [, pencil] = within(cardActions).getAllByRole("button");
    await user.click(pencil);

    const textarea = screen.getByDisplayValue("Старый текст");
    fireEvent.change(textarea, { target: { value: "Новый текст" } });
    await user.click(screen.getByRole("button", { name: "Сохранить" }));

    // The PATCH-equivalent POST carried the new body and the card reflects it.
    await waitFor(() => expect(editBody).toEqual({ body: "Новый текст" }));
    expect(await screen.findByText(/Новый текст/)).toBeInTheDocument();
    expect(screen.queryByText(/Старый текст/)).not.toBeInTheDocument();
  });

  it("rejects a queued message and removes it from the list", async () => {
    const user = userEvent.setup({ delay: null });
    let rejectedId: string | null = null;
    mountWith(
      [
        outbound({ id: "a", prospect_id: "alpha000000" }),
        outbound({ id: "b", prospect_id: "beta0000000" }),
      ],
      [],
      { ...emptyStats, draft: 2 },
      [
        http.post(url("/api/outbound/a/reject"), () => {
          rejectedId = "a";
          return new HttpResponse(null, { status: 204 });
        }),
      ],
    );

    render(<OutboundPage />);
    await screen.findByText("Проспект alpha0");

    // The reject (X) button is the last action button on the card.
    const approve = screen.getAllByRole("button", { name: /Подтвердить$/ })[0];
    const actions = within(approve.parentElement as HTMLElement).getAllByRole("button");
    await user.click(actions[actions.length - 1]);

    expect(await screen.findByText("Проспект beta00")).toBeInTheDocument();
    expect(screen.queryByText("Проспект alpha0")).not.toBeInTheDocument();
    expect(rejectedId).toBe("a");
  });

  it("approves the whole queue at once and empties the list", async () => {
    const user = userEvent.setup({ delay: null });
    const approved: string[] = [];
    mountWith(
      [
        outbound({ id: "a", prospect_id: "alpha000000" }),
        outbound({ id: "b", prospect_id: "beta0000000" }),
      ],
      [],
      { ...emptyStats, draft: 2 },
      [
        http.post(url("/api/outbound/:id/approve"), ({ params }) => {
          approved.push(params.id as string);
          return new HttpResponse(null, { status: 204 });
        }),
      ],
    );

    render(<OutboundPage />);
    await screen.findByText("Проспект alpha0");

    await user.click(screen.getByRole("button", { name: /Подтвердить все/ }));

    // Both messages were approved sequentially and the empty-state shows.
    expect(await screen.findByText("Нет сообщений в очереди")).toBeInTheDocument();
    await waitFor(() => expect(approved.sort()).toEqual(["a", "b"]));
  });

  it("shows sent messages on the sent tab and filters them by status", async () => {
    mountWith(
      [],
      [
        outbound({ id: "s1", prospect_id: "sentaa0000", status: "sent" }),
        outbound({ id: "r1", prospect_id: "rejbb00000", status: "rejected" }),
      ],
    );

    render(<OutboundPage />);
    // Queue starts empty.
    await screen.findByText("Нет сообщений в очереди");

    fireEvent.click(screen.getByRole("button", { name: /Отправленные/ }));

    // Both sent-tab rows render initially.
    expect(await screen.findByText("Проспект sentaa")).toBeInTheDocument();
    expect(screen.getByText("Проспект rejbb0")).toBeInTheDocument();

    // Status filter "Отправлено" keeps only the sent message.
    fireEvent.click(screen.getByRole("button", { name: "Отправлено" }));

    expect(screen.getByText("Проспект sentaa")).toBeInTheDocument();
    expect(screen.queryByText("Проспект rejbb0")).not.toBeInTheDocument();
  });

  it("filters the queue by the search box", async () => {
    mountWith([
      outbound({ id: "a", prospect_id: "alpha000000", body: "первое" }),
      outbound({ id: "b", prospect_id: "beta0000000", body: "второе" }),
    ]);

    render(<OutboundPage />);
    await screen.findByText("Проспект alpha0");

    fireEvent.change(screen.getByPlaceholderText("Поиск по очереди..."), {
      target: { value: "beta" },
    });

    expect(await screen.findByText("Проспект beta00")).toBeInTheDocument();
    expect(screen.queryByText("Проспект alpha0")).not.toBeInTheDocument();
  });

  it("reflects the autopilot status read from the settings API", async () => {
    // The autopilot banner is hydrated from GET /api/settings.auto_send.
    server.use(
      http.get(url("/api/outbound/queue"), () => HttpResponse.json([])),
      http.get(url("/api/outbound/sent"), () => HttpResponse.json([])),
      http.get(url("/api/outbound/stats"), () => HttpResponse.json(emptyStats)),
      http.get(url("/api/settings"), () => HttpResponse.json({ auto_send: true })),
    );

    render(<OutboundPage />);

    expect(
      await screen.findByText(
        "Включён: сообщения отправляются автоматически, без ручного одобрения",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Вкл")).toBeInTheDocument();
  });

  it("keeps the card in the queue when approve fails", async () => {
    const user = userEvent.setup({ delay: null });
    mountWith(
      [outbound({ id: "a", prospect_id: "alpha000000" })],
      [],
      { ...emptyStats, draft: 1 },
      [
        http.post(url("/api/outbound/a/approve"), () =>
          HttpResponse.json({ error: "boom" }, { status: 500 }),
        ),
      ],
    );

    render(<OutboundPage />);
    await screen.findByText("Проспект alpha0");

    await user.click(screen.getByRole("button", { name: /Подтвердить$/ }));

    // The failed approve is swallowed; the row stays visible.
    await waitFor(() =>
      expect(screen.getByText("Проспект alpha0")).toBeInTheDocument(),
    );
  });
});
