import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi, describe, it, expect, beforeEach } from "vitest";

/* ---- Mocks ---- */

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), back: vi.fn() }),
  usePathname: () => "/pipeline",
}));

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: any) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

const mockGetLeads = vi.fn();
const mockGetQualification = vi.fn();
const mockUpdateLeadStatus = vi.fn();

vi.mock("@/lib/api", () => ({
  api: {
    getLeads: (...args: any[]) => mockGetLeads(...args),
    getQualification: (...args: any[]) => mockGetQualification(...args),
    updateLeadStatus: (...args: any[]) => mockUpdateLeadStatus(...args),
  },
  // Re-export Lead type placeholder (not needed at runtime, just for imports)
}));

vi.mock("@/lib/utils", () => ({
  cn: (...args: any[]) => args.filter(Boolean).join(" "),
}));

import PipelinePage from "./page";

/* ---- Helpers ---- */

function makeLead(overrides: Partial<{
  id: string;
  contact_name: string;
  company: string;
  channel: string;
  first_message: string;
  status: string;
  created_at: string;
}> = {}) {
  return {
    id: overrides.id ?? "lead-1",
    user_id: "u1",
    contact_name: overrides.contact_name ?? "Иван Петров",
    company: overrides.company ?? "ООО Тест",
    channel: overrides.channel ?? "telegram",
    first_message: overrides.first_message ?? "Привет",
    status: overrides.status ?? "new",
    created_at: overrides.created_at ?? new Date().toISOString(),
    updated_at: new Date().toISOString(),
  };
}

/* ---- Tests ---- */

describe("PipelinePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetQualification.mockResolvedValue(null);
  });

  it("renders all kanban columns", async () => {
    mockGetLeads.mockResolvedValue([]);

    render(<PipelinePage />);

    await waitFor(() => {
      expect(screen.getByText("Новый")).toBeInTheDocument();
    });
    expect(screen.getByText("Квалифицирован")).toBeInTheDocument();
    expect(screen.getByText("В диалоге")).toBeInTheDocument();
    expect(screen.getByText("Фоллоуап")).toBeInTheDocument();
    expect(screen.getByText("Выигран")).toBeInTheDocument();
    expect(screen.getByText("Закрыт")).toBeInTheDocument();
  });

  it("renders page header", async () => {
    mockGetLeads.mockResolvedValue([]);

    render(<PipelinePage />);

    await waitFor(() => {
      expect(screen.getByText("Воронка продаж")).toBeInTheDocument();
    });
  });

  it("distributes leads into correct columns", async () => {
    mockGetLeads.mockResolvedValue([
      makeLead({ id: "l1", contact_name: "Новый Лид", status: "new" }),
      makeLead({ id: "l2", contact_name: "Квал Лид", status: "qualified" }),
      makeLead({ id: "l3", contact_name: "Закрытый Лид", status: "closed" }),
    ]);

    render(<PipelinePage />);

    await waitFor(() => {
      expect(screen.getByText("Новый Лид")).toBeInTheDocument();
    });
    expect(screen.getByText("Квал Лид")).toBeInTheDocument();
    expect(screen.getByText("Закрытый Лид")).toBeInTheDocument();
  });

  it("shows column counts", async () => {
    mockGetLeads.mockResolvedValue([
      makeLead({ id: "l1", status: "new" }),
      makeLead({ id: "l2", status: "new" }),
      makeLead({ id: "l3", status: "qualified" }),
    ]);

    render(<PipelinePage />);

    // Column count badges: "new" column should show "2", "qualified" should show "1"
    await waitFor(() => {
      // Find column badges by their badge style class
      const badges = screen.getAllByText("2");
      expect(badges.length).toBeGreaterThanOrEqual(1);
    });
  });

  it("shows metric cards with correct data", async () => {
    mockGetLeads.mockResolvedValue([
      makeLead({ id: "l1", status: "new" }),
      makeLead({ id: "l2", status: "new" }),
      makeLead({ id: "l3", status: "qualified" }),
      makeLead({ id: "l4", status: "closed" }),
    ]);

    render(<PipelinePage />);

    await waitFor(() => {
      // Total active = all except closed = 3
      expect(screen.getByText("3")).toBeInTheDocument();
    });
    // Conversion: qualified(1) / new(2) = 50%
    expect(screen.getByText("50%")).toBeInTheDocument();
  });

  it("filters by channel", async () => {
    mockGetLeads.mockResolvedValue([
      makeLead({ id: "l1", contact_name: "TG Лид", channel: "telegram", status: "new" }),
      makeLead({ id: "l2", contact_name: "Email Лид", channel: "email", status: "new" }),
    ]);

    const user = userEvent.setup();
    render(<PipelinePage />);

    await waitFor(() => {
      expect(screen.getByText("TG Лид")).toBeInTheDocument();
      expect(screen.getByText("Email Лид")).toBeInTheDocument();
    });

    // Click Telegram filter button (in the channel filter bar)
    const filterButtons = screen.getAllByText("Telegram");
    // The filter button is a <button>, the card badge is a <span>
    const telegramFilterBtn = filterButtons.find((el) => el.tagName === "BUTTON")!;
    await user.click(telegramFilterBtn);

    await waitFor(() => {
      expect(screen.getByText("TG Лид")).toBeInTheDocument();
      expect(screen.queryByText("Email Лид")).not.toBeInTheDocument();
    });
  });

  it("shows AI insight about followups", async () => {
    mockGetLeads.mockResolvedValue([
      makeLead({ id: "l1", status: "followup" }),
      makeLead({ id: "l2", status: "followup" }),
    ]);

    render(<PipelinePage />);

    await waitFor(() => {
      expect(
        screen.getByText(/2 сделок в «Фоллоуап» требуют срочного внимания/),
      ).toBeInTheDocument();
    });
  });

  it("shows AI insight when no followups", async () => {
    mockGetLeads.mockResolvedValue([
      makeLead({ id: "l1", status: "new" }),
    ]);

    render(<PipelinePage />);

    await waitFor(() => {
      expect(
        screen.getByText("Все лиды в работе, фоллоуапов нет"),
      ).toBeInTheDocument();
    });
  });

  it("shows company on lead card when present", async () => {
    mockGetLeads.mockResolvedValue([
      makeLead({ id: "l1", contact_name: "Иван", company: "МегаКорп", status: "new" }),
    ]);

    render(<PipelinePage />);

    await waitFor(() => {
      expect(screen.getByText("Иван")).toBeInTheDocument();
      expect(screen.getByText("МегаКорп")).toBeInTheDocument();
    });
  });
});
