import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";

import { server, url } from "@/__tests__/integration/server";
import type { PendingReplyQueueRow } from "@/lib/api";

// PendingQueueTabs (child) reads usePathname; pin it so the page mounts
// deterministically without a Next router context.
vi.mock("next/navigation", () => ({
  usePathname: () => "/inbox/pending",
}));

import InboxPendingPage from "./page";

// Minimal valid HITL queue row; override per test. Defaults to a telegram
// booking_link draft in pending status.
function row(over: Partial<PendingReplyQueueRow> = {}): PendingReplyQueueRow {
  return {
    id: over.id ?? "pr-1",
    lead_id: over.lead_id ?? "lead-1",
    channel: over.channel ?? "telegram",
    kind: over.kind ?? "booking_link",
    body: over.body ?? "Здравствуйте! Вот ссылка на встречу: cal.com/floq",
    status: over.status ?? "pending",
    created_at: over.created_at ?? "2026-06-25T10:00:00Z",
    lead: over.lead ?? {
      contact_name: "Иван Петров",
      company: "Acme",
      channel: over.channel ?? "telegram",
    },
  };
}

// Integration: real InboxPendingPage + usePendingQueue + lib/api.ts, network
// via MSW. On mount the page fires exactly one GET:
//   /api/pending-replies?status=pending  (the HITL approval queue)
function mountWith(rows: PendingReplyQueueRow[], extra: Parameters<typeof server.use> = []) {
  server.use(
    http.get(url("/api/pending-replies"), () => HttpResponse.json(rows)),
    ...extra,
  );
}

describe("inbox pending queue page (integration)", () => {
  it("loads the pending queue from the API and renders it", async () => {
    mountWith([
      row({ id: "a", body: "Драфт А", lead: { contact_name: "Иван Петров", company: "Acme", channel: "telegram" } }),
      row({ id: "b", body: "Драфт Б", lead: { contact_name: "Мария Сидорова", company: "Globex", channel: "email" } }),
    ]);

    render(<InboxPendingPage />);

    expect(await screen.findByText("Иван Петров · Acme")).toBeInTheDocument();
    expect(screen.getByText("Мария Сидорова · Globex")).toBeInTheDocument();
    expect(screen.getByText("Драфт А")).toBeInTheDocument();
    expect(screen.getByText("Драфт Б")).toBeInTheDocument();
    // The HITL tab badge reflects the loaded count.
    expect(screen.getByText("Очередь HITL (2)")).toBeInTheDocument();
  });

  it("shows the empty state when the queue is empty", async () => {
    mountWith([]);

    render(<InboxPendingPage />);

    expect(await screen.findByText("Нет драфтов на одобрение")).toBeInTheDocument();
  });

  it("filters the rendered rows by the selected channel", async () => {
    mountWith([
      row({ id: "a", channel: "telegram", lead: { contact_name: "Иван Петров", company: "Acme", channel: "telegram" } }),
      row({ id: "b", channel: "email", lead: { contact_name: "Мария Сидорова", company: "Globex", channel: "email" } }),
    ]);

    render(<InboxPendingPage />);
    await screen.findByText("Иван Петров · Acme");

    // Narrow to Email -> the telegram row drops out.
    fireEvent.click(screen.getByText("Telegram"));

    expect(screen.queryByText("Мария Сидорова · Globex")).not.toBeInTheDocument();
    expect(screen.getByText("Иван Петров · Acme")).toBeInTheDocument();
  });

  it("approves a pending reply via POST and removes it from the queue", async () => {
    const user = userEvent.setup({ delay: null });
    let approvedId: string | null = null;
    mountWith(
      [
        row({ id: "a", lead: { contact_name: "Иван Петров", company: "Acme", channel: "telegram" } }),
        row({ id: "b", lead: { contact_name: "Мария Сидорова", company: "Globex", channel: "telegram" } }),
      ],
      [
        http.post(url("/api/pending-replies/a/approve"), () => {
          approvedId = "a";
          return new HttpResponse(null, { status: 204 });
        }),
      ],
    );

    render(<InboxPendingPage />);
    await screen.findByText("Иван Петров · Acme");

    // First row's approve button.
    await user.click(screen.getAllByRole("button", { name: /Одобрить и отправить/ })[0]);

    expect(approvedId).toBe("a");
    // Optimistic removal of the approved row; the other stays.
    expect(await screen.findByText("Очередь HITL (1)")).toBeInTheDocument();
    expect(screen.queryByText("Иван Петров · Acme")).not.toBeInTheDocument();
    expect(screen.getByText("Мария Сидорова · Globex")).toBeInTheDocument();
  });

  it("rejects a pending reply via POST and removes it from the queue", async () => {
    const user = userEvent.setup({ delay: null });
    let rejectedId: string | null = null;
    mountWith(
      [row({ id: "a", lead: { contact_name: "Иван Петров", company: "Acme", channel: "telegram" } })],
      [
        http.post(url("/api/pending-replies/a/reject"), () => {
          rejectedId = "a";
          return new HttpResponse(null, { status: 204 });
        }),
      ],
    );

    render(<InboxPendingPage />);
    await screen.findByText("Иван Петров · Acme");

    await user.click(screen.getByRole("button", { name: /Отклонить/ }));

    expect(rejectedId).toBe("a");
    expect(await screen.findByText("Нет драфтов на одобрение")).toBeInTheDocument();
  });

  it("bulk-approves selected rows via POST and reports the summary", async () => {
    const user = userEvent.setup({ delay: null });
    let bulkBody: { ids: string[]; decision: string } | null = null;
    mountWith(
      [
        row({ id: "a", lead: { contact_name: "Иван Петров", company: "Acme", channel: "telegram" } }),
        row({ id: "b", lead: { contact_name: "Мария Сидорова", company: "Globex", channel: "telegram" } }),
      ],
      [
        http.post(url("/api/pending-replies/bulk"), async ({ request }) => {
          bulkBody = (await request.json()) as { ids: string[]; decision: string };
          return HttpResponse.json({
            results: bulkBody.ids.map((id) => ({ id, ok: true })),
          });
        }),
      ],
    );

    render(<InboxPendingPage />);
    await screen.findByText("Иван Петров · Acme");

    // Select both rows via their checkboxes.
    await user.click(screen.getByLabelText("Выбрать драфт для Иван Петров"));
    await user.click(screen.getByLabelText("Выбрать драфт для Мария Сидорова"));

    expect(screen.getByText("Выбрано: 2")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /Одобрить выбранные/ }));

    expect(bulkBody).toEqual({ ids: ["a", "b"], decision: "approve" });
    expect(await screen.findByText("Готово: 2 применено")).toBeInTheDocument();
    // Both rows removed -> empty state.
    expect(screen.getByText("Нет драфтов на одобрение")).toBeInTheDocument();
  });
});
