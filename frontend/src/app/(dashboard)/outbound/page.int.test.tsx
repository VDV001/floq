import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
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
});
