import type { ReactNode } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi, describe, it, expect, beforeEach } from "vitest";

/* ---- Mocks ---- */

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), back: vi.fn() }),
  usePathname: () => "/inbox",
}));

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

const mockGetLeads = vi.fn();
const mockGetQualification = vi.fn();
const mockExportLeadsCSV = vi.fn();
const mockImportLeadsCSV = vi.fn();
const mockGetSuggestionCounts = vi.fn();

vi.mock("@/lib/api", () => ({
  api: {
    getLeads: (...args: unknown[]) => mockGetLeads(...args),
    getQualification: (...args: unknown[]) => mockGetQualification(...args),
    exportLeadsCSV: (...args: unknown[]) => mockExportLeadsCSV(...args),
    importLeadsCSV: (...args: unknown[]) => mockImportLeadsCSV(...args),
    getSuggestionCounts: (...args: unknown[]) => mockGetSuggestionCounts(...args),
  },
}));

vi.mock("@/lib/utils", () => ({
  cn: (...args: unknown[]) => args.filter(Boolean).join(" "),
}));

import InboxPage from "./page";

/* ---- Helpers ---- */

function makeLead(overrides: Partial<{
  id: string;
  contact_name: string;
  company: string;
  channel: string;
  first_message: string;
  status: string;
  source_name: string;
  created_at: string;
}> = {}) {
  return {
    id: overrides.id ?? "lead-1",
    user_id: "u1",
    contact_name: overrides.contact_name ?? "Иван Петров",
    company: overrides.company ?? "ООО Ромашка",
    channel: overrides.channel ?? "telegram",
    first_message: overrides.first_message ?? "Здравствуйте, нужна CRM",
    status: overrides.status ?? "new",
    source_name: overrides.source_name,
    created_at: overrides.created_at ?? new Date().toISOString(),
    updated_at: new Date().toISOString(),
  };
}

/* ---- Tests ---- */

describe("InboxPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetQualification.mockResolvedValue(null);
    mockGetSuggestionCounts.mockResolvedValue({});
  });

  it("renders empty state when no leads", async () => {
    mockGetLeads.mockResolvedValue([]);

    render(<InboxPage />);

    await waitFor(() => {
      expect(screen.getByText("Нет лидов")).toBeInTheDocument();
    });
    expect(
      screen.getByText("Напишите вашему Telegram боту чтобы создать первый лид"),
    ).toBeInTheDocument();
  });

  it("renders lead cards with mapped status and data", async () => {
    mockGetLeads.mockResolvedValue([
      makeLead({ id: "l1", contact_name: "Анна Сидорова", company: "Тест Компания", status: "new" }),
      makeLead({ id: "l2", contact_name: "Борис Иванов", company: "Другая Компания", status: "qualified" }),
    ]);

    render(<InboxPage />);

    await waitFor(() => {
      expect(screen.getByText("Тест Компания")).toBeInTheDocument();
    });
    // contact_name is rendered inside "через Telegram · {contact}"
    expect(screen.getByText(/Анна Сидорова/)).toBeInTheDocument();
    // Lead with status "qualified" should NOT be visible (filtered by pipeline stage "new")
    expect(screen.queryByText("Другая Компания")).not.toBeInTheDocument();
  });

  it("switches pipeline stage and shows corresponding leads", async () => {
    mockGetLeads.mockResolvedValue([
      makeLead({ id: "l1", status: "new", company: "Новая Компания" }),
      makeLead({ id: "l2", status: "qualified", company: "Квал Компания" }),
    ]);

    const user = userEvent.setup();
    render(<InboxPage />);

    await waitFor(() => {
      expect(screen.getByText("Новая Компания")).toBeInTheDocument();
    });

    // Click on "Квалифицированные" stage
    await user.click(screen.getByText("Квалифицированные"));

    await waitFor(() => {
      expect(screen.getByText("Квал Компания")).toBeInTheDocument();
    });
    expect(screen.queryByText("Новая Компания")).not.toBeInTheDocument();
  });

  it("filters leads by source", async () => {
    mockGetLeads.mockResolvedValue([
      makeLead({ id: "l1", status: "new", company: "Компания A", source_name: "LinkedIn" }),
      makeLead({ id: "l2", status: "new", company: "Компания B", source_name: "Google" }),
    ]);

    const user = userEvent.setup();
    render(<InboxPage />);

    await waitFor(() => {
      expect(screen.getByText("Компания A")).toBeInTheDocument();
    });
    expect(screen.getByText("Компания B")).toBeInTheDocument();

    // Select source filter
    const sourceSelect = screen.getByDisplayValue("Все источники");
    await user.selectOptions(sourceSelect, "LinkedIn");

    await waitFor(() => {
      expect(screen.getByText("Компания A")).toBeInTheDocument();
      expect(screen.queryByText("Компания B")).not.toBeInTheDocument();
    });
  });

  it("displays pipeline stage counts", async () => {
    mockGetLeads.mockResolvedValue([
      makeLead({ id: "l1", status: "new" }),
      makeLead({ id: "l2", status: "new" }),
      makeLead({ id: "l3", status: "qualified" }),
    ]);

    render(<InboxPage />);

    await waitFor(() => {
      // "Новые лиды" stage should show count 2
      const stageButtons = screen.getAllByRole("button");
      const newStageBtn = stageButtons.find((b) => b.textContent?.includes("Новые лиды"));
      expect(newStageBtn?.textContent).toContain("2");
    });
  });

  it("shows AI summary with lead count", async () => {
    mockGetLeads.mockResolvedValue([
      makeLead({ id: "l1", status: "new" }),
    ]);

    render(<InboxPage />);

    await waitFor(() => {
      expect(screen.getByText(/1 лид в системе/)).toBeInTheDocument();
    });
  });

  it("loads qualification for /start messages", async () => {
    mockGetLeads.mockResolvedValue([
      makeLead({ id: "l1", first_message: "/start", status: "new" }),
    ]);
    mockGetQualification.mockResolvedValue({ identified_need: "Нужна автоматизация" });

    render(<InboxPage />);

    await waitFor(() => {
      expect(mockGetQualification).toHaveBeenCalledWith("l1");
    });

    await waitFor(() => {
      expect(screen.getByText("Нужна автоматизация")).toBeInTheDocument();
    });
  });

  it("renders lead links with correct href", async () => {
    mockGetLeads.mockResolvedValue([
      makeLead({ id: "lead-42", status: "new" }),
    ]);

    render(<InboxPage />);

    await waitFor(() => {
      const link = screen.getByRole("link");
      expect(link).toHaveAttribute("href", "/inbox/lead-42");
    });
  });
});
