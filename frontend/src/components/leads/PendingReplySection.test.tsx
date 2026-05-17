import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { PendingReplySection } from "./PendingReplySection";
import { api, type PendingReply } from "@/lib/api";

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    api: {
      getPendingReplies: vi.fn(),
      approvePendingReply: vi.fn(),
      rejectPendingReply: vi.fn(),
    },
  };
});

const reply = (over: Partial<PendingReply> = {}): PendingReply => ({
  id: "pr-1",
  lead_id: "lead-1",
  channel: "telegram",
  kind: "booking_link",
  body: "Отлично! Вот ссылка: https://cal.com/x",
  status: "pending",
  created_at: "2026-05-17T18:00:00Z",
  ...over,
});

describe("PendingReplySection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing while the fetch is in-flight", () => {
    vi.mocked(api.getPendingReplies).mockReturnValue(new Promise(() => {}));
    const { container } = render(<PendingReplySection leadId="lead-1" />);
    expect(container.firstChild).toBeNull();
  });

  it("renders nothing when there are no pending rows", async () => {
    vi.mocked(api.getPendingReplies).mockResolvedValue([]);
    const { container } = render(<PendingReplySection leadId="lead-1" />);
    await waitFor(() => expect(api.getPendingReplies).toHaveBeenCalledWith("lead-1"));
    expect(container.firstChild).toBeNull();
  });

  it("renders nothing when every reply is already decided (sent/rejected/approved)", async () => {
    vi.mocked(api.getPendingReplies).mockResolvedValue([
      reply({ id: "a", status: "sent" }),
      reply({ id: "b", status: "rejected" }),
      reply({ id: "c", status: "approved" }),
    ]);
    const { container } = render(<PendingReplySection leadId="lead-1" />);
    await waitFor(() => expect(api.getPendingReplies).toHaveBeenCalled());
    expect(container.firstChild).toBeNull();
  });

  it("renders pending body with kind + channel labels", async () => {
    vi.mocked(api.getPendingReplies).mockResolvedValue([reply()]);
    render(<PendingReplySection leadId="lead-1" />);

    expect(await screen.findByText(/Отлично! Вот ссылка/)).toBeInTheDocument();
    expect(screen.getByText(/Ссылка на встречу/i)).toBeInTheDocument();
    expect(screen.getByText(/Telegram/i)).toBeInTheDocument();
  });

  it("approves on click, removes the row, and calls onApproved", async () => {
    const user = userEvent.setup();
    vi.mocked(api.getPendingReplies).mockResolvedValue([reply()]);
    vi.mocked(api.approvePendingReply).mockResolvedValue(undefined);
    const onApproved = vi.fn();
    render(<PendingReplySection leadId="lead-1" onApproved={onApproved} />);

    const btn = await screen.findByRole("button", { name: /Одобрить и отправить/i });
    await user.click(btn);

    await waitFor(() => expect(api.approvePendingReply).toHaveBeenCalledWith("pr-1"));
    await waitFor(() => expect(screen.queryByText(/Отлично! Вот ссылка/)).not.toBeInTheDocument());
    expect(onApproved).toHaveBeenCalledOnce();
  });

  it("rejects on click and removes the row without calling onApproved", async () => {
    const user = userEvent.setup();
    vi.mocked(api.getPendingReplies).mockResolvedValue([reply()]);
    vi.mocked(api.rejectPendingReply).mockResolvedValue(undefined);
    const onApproved = vi.fn();
    render(<PendingReplySection leadId="lead-1" onApproved={onApproved} />);

    const btn = await screen.findByRole("button", { name: /Отклонить/i });
    await user.click(btn);

    await waitFor(() => expect(api.rejectPendingReply).toHaveBeenCalledWith("pr-1"));
    await waitFor(() => expect(screen.queryByText(/Отлично! Вот ссылка/)).not.toBeInTheDocument());
    expect(onApproved).not.toHaveBeenCalled();
  });

  it("surfaces an error message when approve fails and keeps the row", async () => {
    const user = userEvent.setup();
    vi.mocked(api.getPendingReplies).mockResolvedValue([reply()]);
    vi.mocked(api.approvePendingReply).mockRejectedValue(new Error("boom"));
    render(<PendingReplySection leadId="lead-1" />);

    const btn = await screen.findByRole("button", { name: /Одобрить и отправить/i });
    await user.click(btn);

    await waitFor(() => expect(api.approvePendingReply).toHaveBeenCalled());
    expect(await screen.findByRole("alert")).toHaveTextContent(/не удалось одобрить/i);
    // Row still present after failure so the operator can retry.
    expect(screen.getByText(/Отлично! Вот ссылка/)).toBeInTheDocument();
  });

  it("disables both buttons while a decision is in flight", async () => {
    const user = userEvent.setup();
    vi.mocked(api.getPendingReplies).mockResolvedValue([reply()]);
    let resolve: (v: void) => void = () => {};
    vi.mocked(api.approvePendingReply).mockReturnValue(
      new Promise((r) => {
        resolve = r;
      }),
    );
    render(<PendingReplySection leadId="lead-1" />);

    const approveBtn = await screen.findByRole("button", { name: /Одобрить и отправить/i });
    const rejectBtn = screen.getByRole("button", { name: /Отклонить/i });

    await user.click(approveBtn);
    expect(approveBtn).toBeDisabled();
    expect(rejectBtn).toBeDisabled();

    resolve();
  });

  it("renders nothing when the fetch fails", async () => {
    vi.mocked(api.getPendingReplies).mockRejectedValue(new Error("boom"));
    const { container } = render(<PendingReplySection leadId="lead-1" />);
    await waitFor(() => expect(api.getPendingReplies).toHaveBeenCalled());
    expect(container.firstChild).toBeNull();
  });
});
