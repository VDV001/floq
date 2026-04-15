import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import AlertsPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), back: vi.fn() }),
  usePathname: () => "/alerts",
}));

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: any) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const mockLeads = [
  {
    id: "lead-1",
    user_id: "u1",
    channel: "telegram" as const,
    contact_name: "Иван Петров",
    company: "Acme Corp",
    first_message: "Интересует ваше предложение по автоматизации продаж",
    status: "followup" as const,
    telegram_chat_id: 123,
    created_at: "2026-04-01T10:00:00Z",
    updated_at: "2026-04-08T10:00:00Z",
  },
  {
    id: "lead-2",
    user_id: "u1",
    channel: "email" as const,
    contact_name: "Мария Сидорова",
    company: "Beta Inc",
    first_message: "Хотели бы обсудить интеграцию",
    status: "followup" as const,
    email_address: "maria@beta.com",
    created_at: "2026-04-02T10:00:00Z",
    updated_at: "2026-04-10T10:00:00Z",
  },
  {
    id: "lead-3",
    user_id: "u1",
    channel: "telegram" as const,
    contact_name: "Алексей",
    company: "",
    first_message: "",
    status: "new" as const,
    created_at: "2026-04-05T10:00:00Z",
    updated_at: "2026-04-12T10:00:00Z",
  },
];

vi.mock("@/lib/api", () => ({
  api: {
    getLeads: vi.fn(),
  },
}));

import { api } from "@/lib/api";

describe("AlertsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders alerts for followup leads", async () => {
    vi.mocked(api.getLeads).mockResolvedValue(mockLeads as any);

    render(<AlertsPage />);

    await waitFor(() => {
      expect(screen.getByText("Напоминания")).toBeInTheDocument();
      expect(screen.getByText("Иван Петров")).toBeInTheDocument();
    });
  });

  it("shows empty state when no followup leads", async () => {
    vi.mocked(api.getLeads).mockResolvedValue([
      { ...mockLeads[2], status: "new" as const },
    ] as any);

    render(<AlertsPage />);

    await waitFor(() => {
      expect(screen.getByText("Нет напоминаний")).toBeInTheDocument();
    });
  });

  it("renders alert summary section", async () => {
    vi.mocked(api.getLeads).mockResolvedValue(mockLeads as any);

    render(<AlertsPage />);

    await waitFor(() => {
      expect(screen.getByText("Сводка алертов")).toBeInTheDocument();
    });
  });

  it("shows featured card for the first followup lead", async () => {
    vi.mocked(api.getLeads).mockResolvedValue(mockLeads as any);

    render(<AlertsPage />);

    await waitFor(() => {
      expect(screen.getByText("Отправить напоминание")).toBeInTheDocument();
      expect(screen.getByText("Иван Петров")).toBeInTheDocument();
    });
  });
});
