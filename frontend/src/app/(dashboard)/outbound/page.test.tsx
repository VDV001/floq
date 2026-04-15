import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import OutboundPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), back: vi.fn() }),
  usePathname: () => "/outbound",
}));

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: any) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock("@/components/ui/switch", () => ({
  Switch: ({ checked, onCheckedChange, ...props }: any) => (
    <button
      role="switch"
      aria-checked={checked}
      onClick={() => onCheckedChange?.(!checked)}
      {...props}
    />
  ),
}));

const mockQueue = [
  {
    id: "msg-1",
    prospect_id: "abcdef123456",
    sequence_id: "seq-111111",
    step_order: 1,
    channel: "email" as const,
    body: "Здравствуйте! Хотел бы обсудить...",
    status: "draft" as const,
    scheduled_at: new Date().toISOString(),
    sent_at: null,
    created_at: new Date().toISOString(),
  },
];

const mockSent = [
  {
    id: "msg-2",
    prospect_id: "xyz789000000",
    sequence_id: "seq-222222",
    step_order: 1,
    channel: "telegram" as const,
    body: "Привет!",
    status: "sent" as const,
    scheduled_at: new Date().toISOString(),
    sent_at: new Date().toISOString(),
    created_at: new Date().toISOString(),
  },
];

const mockStats = {
  draft: 1,
  approved: 0,
  sent: 1,
  opened: 0,
  replied: 0,
  bounced: 0,
};

vi.mock("@/lib/api", () => ({
  api: {
    getOutboundQueue: vi.fn(),
    getOutboundSent: vi.fn(),
    getOutboundStats: vi.fn(),
    approveMessage: vi.fn(),
    rejectMessage: vi.fn(),
    editMessage: vi.fn(),
  },
}));

import { api } from "@/lib/api";

describe("OutboundPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getOutboundQueue).mockResolvedValue(mockQueue as any);
    vi.mocked(api.getOutboundSent).mockResolvedValue(mockSent as any);
    vi.mocked(api.getOutboundStats).mockResolvedValue(mockStats as any);
  });

  it("renders page header", async () => {
    render(<OutboundPage />);

    await waitFor(() => {
      expect(screen.getByText("Очередь отправки")).toBeInTheDocument();
    });
  });

  it("renders queue messages", async () => {
    render(<OutboundPage />);

    await waitFor(() => {
      expect(screen.getByText(/Здравствуйте/)).toBeInTheDocument();
    });
  });

  it("renders stats", async () => {
    render(<OutboundPage />);

    await waitFor(() => {
      expect(screen.getByText("В очереди")).toBeInTheDocument();
      expect(screen.getByText("Отправлено")).toBeInTheDocument();
    });
  });

  it("approves a message", async () => {
    const user = userEvent.setup();
    vi.mocked(api.approveMessage).mockResolvedValue(undefined as any);

    render(<OutboundPage />);

    await waitFor(() => {
      expect(screen.getByText("Подтвердить")).toBeInTheDocument();
    });

    await user.click(screen.getByText("Подтвердить"));

    await waitFor(() => {
      expect(api.approveMessage).toHaveBeenCalledWith("msg-1");
    });
  });

  it("shows empty state when no messages", async () => {
    vi.mocked(api.getOutboundQueue).mockResolvedValue([]);
    vi.mocked(api.getOutboundSent).mockResolvedValue([]);

    render(<OutboundPage />);

    await waitFor(() => {
      expect(screen.getByText("Нет сообщений в очереди")).toBeInTheDocument();
    });
  });
});
