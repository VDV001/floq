import type { ReactNode } from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { PendingReplyQueueRow } from "@/lib/api";
import InboxPendingPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), back: vi.fn() }),
  usePathname: () => "/inbox/pending",
}));

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

vi.mock("@/lib/api", () => ({
  api: {
    listPendingReplies: vi.fn(),
    approvePendingReply: vi.fn(),
    rejectPendingReply: vi.fn(),
  },
}));

import { api } from "@/lib/api";

function makeRow(over: Partial<PendingReplyQueueRow> = {}): PendingReplyQueueRow {
  return {
    id: over.id ?? "pr-1",
    lead_id: over.lead_id ?? "lead-1",
    channel: over.channel ?? "telegram",
    kind: over.kind ?? "booking_link",
    body: over.body ?? "Hi, here's my booking link",
    status: "pending",
    created_at: over.created_at ?? "2026-05-20T10:00:00Z",
    lead: over.lead ?? {
      contact_name: "Иван Петров",
      company: "ACME",
      channel: "telegram",
      telegram_chat_id: 123,
    },
  };
}

describe("InboxPendingPage", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(api.listPendingReplies).mockResolvedValue([]);
    vi.mocked(api.approvePendingReply).mockResolvedValue(undefined);
    vi.mocked(api.rejectPendingReply).mockResolvedValue(undefined);
  });

  it("fetches the queue on mount and renders rows with lead context", async () => {
    vi.mocked(api.listPendingReplies).mockResolvedValueOnce([
      makeRow({ id: "pr-1", body: "Telegram draft" }),
      makeRow({
        id: "pr-2",
        channel: "email",
        body: "Email draft",
        lead: { contact_name: "Jane Doe", company: "Globex", channel: "email", email_address: "j@globex.com" },
      }),
    ]);

    render(<InboxPendingPage />);

    await waitFor(() => {
      expect(api.listPendingReplies).toHaveBeenCalledTimes(1);
    });
    expect(screen.getByText("Telegram draft")).toBeInTheDocument();
    expect(screen.getByText("Email draft")).toBeInTheDocument();
    // Lead name + company collapse into one line; assert both halves
    // appear so a regression in either column is caught.
    expect(screen.getByText(/Иван Петров/)).toBeInTheDocument();
    expect(screen.getByText(/ACME/)).toBeInTheDocument();
    expect(screen.getByText(/Jane Doe/)).toBeInTheDocument();
    expect(screen.getByText(/Globex/)).toBeInTheDocument();
  });

  it("renders empty state when queue is empty", async () => {
    vi.mocked(api.listPendingReplies).mockResolvedValueOnce([]);

    render(<InboxPendingPage />);

    await waitFor(() => {
      expect(screen.getByText("Нет драфтов на одобрение")).toBeInTheDocument();
    });
  });

  it("approve button calls api.approvePendingReply and optimistically removes the row", async () => {
    vi.mocked(api.listPendingReplies).mockResolvedValueOnce([
      makeRow({ id: "pr-7", body: "Going away" }),
    ]);
    const user = userEvent.setup();

    render(<InboxPendingPage />);

    await waitFor(() => {
      expect(screen.getByText("Going away")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /Одобрить/ }));

    await waitFor(() => {
      expect(api.approvePendingReply).toHaveBeenCalledWith("pr-7");
    });
    // Optimistic removal — the row should disappear without a refetch.
    await waitFor(() => {
      expect(screen.queryByText("Going away")).not.toBeInTheDocument();
    });
  });

  it("reject button calls api.rejectPendingReply", async () => {
    vi.mocked(api.listPendingReplies).mockResolvedValueOnce([
      makeRow({ id: "pr-9", body: "Reject me" }),
    ]);
    const user = userEvent.setup();

    render(<InboxPendingPage />);

    await waitFor(() => {
      expect(screen.getByText("Reject me")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /Отклонить/ }));

    await waitFor(() => {
      expect(api.rejectPendingReply).toHaveBeenCalledWith("pr-9");
    });
  });

  it("channel filter hides rows from the unselected channel", async () => {
    vi.mocked(api.listPendingReplies).mockResolvedValueOnce([
      makeRow({ id: "pr-tg", body: "TG body", channel: "telegram" }),
      makeRow({
        id: "pr-em",
        body: "Email body",
        channel: "email",
        lead: { contact_name: "Eve", company: "Hooli", channel: "email" },
      }),
    ]);
    const user = userEvent.setup();

    render(<InboxPendingPage />);

    await waitFor(() => {
      expect(screen.getByText("TG body")).toBeInTheDocument();
      expect(screen.getByText("Email body")).toBeInTheDocument();
    });

    // Filter pills label "Telegram" / "Email" appear twice — once in
    // the row metadata, once in the filter bar. Find the filter pill
    // by its button role to avoid the row-meta span.
    const tgFilter = screen.getAllByRole("button", { name: /^Telegram$/ })[0];
    await user.click(tgFilter);

    expect(screen.getByText("TG body")).toBeInTheDocument();
    expect(screen.queryByText("Email body")).not.toBeInTheDocument();
  });
});
