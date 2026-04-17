import type { ReactNode } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi, describe, it, expect, beforeEach } from "vitest";

/* ---- Mocks ---- */

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), back: vi.fn() }),
  usePathname: () => "/prospects",
}));

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

const mockGetProspects = vi.fn();
const mockCreateProspect = vi.fn();
const mockGetSources = vi.fn();
const mockGetSourceStats = vi.fn();
const mockVerifyBatch = vi.fn();
const mockExportProspectsCSV = vi.fn();
const mockImportProspectsCSV = vi.fn();
const mockScrapeWebsite = vi.fn();

vi.mock("@/lib/api", () => ({
  api: {
    getProspects: (...args: unknown[]) => mockGetProspects(...args),
    createProspect: (...args: unknown[]) => mockCreateProspect(...args),
    getSources: (...args: unknown[]) => mockGetSources(...args),
    getSourceStats: (...args: unknown[]) => mockGetSourceStats(...args),
    verifyBatch: (...args: unknown[]) => mockVerifyBatch(...args),
    exportProspectsCSV: (...args: unknown[]) => mockExportProspectsCSV(...args),
    importProspectsCSV: (...args: unknown[]) => mockImportProspectsCSV(...args),
    scrapeWebsite: (...args: unknown[]) => mockScrapeWebsite(...args),
  },
}));

vi.mock("@/components/ui/source-combobox", () => ({
  SourceCombobox: ({ value, onChange }: { value: string; onChange: (v: string | null) => void }) => (
    <select
      data-testid="source-combobox"
      value={value ?? ""}
      onChange={(e) => onChange(e.target.value || null)}
    >
      <option value="">Без источника</option>
    </select>
  ),
}));

vi.mock("@/lib/utils", () => ({
  cn: (...args: unknown[]) => args.filter(Boolean).join(" "),
}));

import ProspectsPage from "./page";

/* ---- Helpers ---- */

function makeProspect(overrides: Partial<{
  name: string;
  company: string;
  title: string;
  email: string;
  phone: string;
  whatsapp: string;
  telegram_username: string;
  source_name: string;
  status: string;
  verify_status: string;
  verify_score: number;
}> = {}) {
  return {
    name: overrides.name ?? "Иван Петров",
    company: overrides.company ?? "ООО Тест",
    title: overrides.title ?? "CTO",
    email: overrides.email ?? "ivan@test.com",
    phone: overrides.phone ?? "+79001234567",
    whatsapp: overrides.whatsapp ?? "",
    telegram_username: overrides.telegram_username ?? "",
    source_name: overrides.source_name ?? "",
    status: overrides.status ?? "new",
    verify_status: overrides.verify_status ?? "not_checked",
    verify_score: overrides.verify_score ?? 0,
  };
}

/* ---- Tests ---- */

describe("ProspectsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetSourceStats.mockResolvedValue([]);
  });

  it("renders empty state when no prospects", async () => {
    mockGetProspects.mockResolvedValue([]);

    render(<ProspectsPage />);

    await waitFor(() => {
      expect(screen.getByText("Нет проспектов")).toBeInTheDocument();
    });
  });

  it("renders table with prospect data", async () => {
    mockGetProspects.mockResolvedValue([
      makeProspect({ name: "Анна Сидорова", company: "BigCorp", title: "CEO", email: "anna@big.com" }),
    ]);

    render(<ProspectsPage />);

    await waitFor(() => {
      expect(screen.getByText("Анна Сидорова")).toBeInTheDocument();
    });
    expect(screen.getByText("BigCorp")).toBeInTheDocument();
    expect(screen.getByText("CEO")).toBeInTheDocument();
    expect(screen.getByText("anna@big.com")).toBeInTheDocument();
    expect(screen.getByText("Новый")).toBeInTheDocument();
    expect(screen.getByText(/Не проверен/)).toBeInTheDocument();
  });

  it("filters prospects by search query", async () => {
    mockGetProspects.mockResolvedValue([
      makeProspect({ name: "Анна Сидорова", company: "BigCorp", email: "anna@big.com" }),
      makeProspect({ name: "Борис Иванов", company: "SmallCorp", email: "boris@small.com" }),
    ]);

    const user = userEvent.setup();
    render(<ProspectsPage />);

    await waitFor(() => {
      expect(screen.getByText("Анна Сидорова")).toBeInTheDocument();
    });
    expect(screen.getByText("Борис Иванов")).toBeInTheDocument();

    const searchInput = screen.getByPlaceholderText("Поиск проспектов...");
    await user.type(searchInput, "Борис");

    await waitFor(() => {
      expect(screen.getByText("Борис Иванов")).toBeInTheDocument();
      expect(screen.queryByText("Анна Сидорова")).not.toBeInTheDocument();
    });
  });

  it("filters prospects by source", async () => {
    mockGetProspects.mockResolvedValue([
      makeProspect({ name: "Анна", source_name: "LinkedIn" }),
      makeProspect({ name: "Борис", source_name: "Google", email: "boris@g.com" }),
    ]);

    const user = userEvent.setup();
    render(<ProspectsPage />);

    await waitFor(() => {
      expect(screen.getByText("Анна")).toBeInTheDocument();
    });

    const sourceSelect = screen.getByDisplayValue("Все источники");
    await user.selectOptions(sourceSelect, "LinkedIn");

    await waitFor(() => {
      expect(screen.getByText("Анна")).toBeInTheDocument();
      expect(screen.queryByText("Борис")).not.toBeInTheDocument();
    });
  });

  it("adds a prospect via form", async () => {
    mockGetProspects.mockResolvedValue([]);
    mockCreateProspect.mockResolvedValue({});

    const user = userEvent.setup();
    render(<ProspectsPage />);

    await waitFor(() => {
      expect(screen.getByText("Нет проспектов")).toBeInTheDocument();
    });

    const nameInput = screen.getByPlaceholderText("Введите имя");
    const companyInput = screen.getByPlaceholderText("Название компании");
    const emailInput = screen.getByPlaceholderText("email@example.com");

    await user.type(nameInput, "Новый Контакт");
    await user.type(companyInput, "Новая Компания");
    await user.type(emailInput, "new@company.com");

    // After form fill, mockGetProspects returns updated list
    mockGetProspects.mockResolvedValue([
      makeProspect({ name: "Новый Контакт", company: "Новая Компания", email: "new@company.com" }),
    ]);

    await user.click(screen.getByText("Добавить"));

    await waitFor(() => {
      expect(mockCreateProspect).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Новый Контакт",
          company: "Новая Компания",
          email: "new@company.com",
        }),
      );
    });
  });

  it("shows correct total count", async () => {
    mockGetProspects.mockResolvedValue([
      makeProspect({ name: "A", email: "a@t.com" }),
      makeProspect({ name: "B", email: "b@t.com" }),
      makeProspect({ name: "C", email: "c@t.com" }),
    ]);

    render(<ProspectsPage />);

    await waitFor(() => {
      expect(screen.getByText("3 контактов")).toBeInTheDocument();
    });
  });

  it("paginates when more than 15 prospects", async () => {
    const manyProspects = Array.from({ length: 20 }, (_, i) =>
      makeProspect({ name: `Person ${i + 1}`, email: `p${i + 1}@t.com` }),
    );
    mockGetProspects.mockResolvedValue(manyProspects);

    const user = userEvent.setup();
    render(<ProspectsPage />);

    await waitFor(() => {
      expect(screen.getByText(/1–15 из 20/)).toBeInTheDocument();
    });

    // Page 2 button should be visible
    const page2Btn = screen.getByText("2");
    await user.click(page2Btn);

    await waitFor(() => {
      expect(screen.getByText(/16–20 из 20/)).toBeInTheDocument();
    });
  });

  it("maps prospect statuses correctly", async () => {
    mockGetProspects.mockResolvedValue([
      makeProspect({ name: "A", email: "a@t.com", status: "in_sequence" }),
    ]);

    render(<ProspectsPage />);

    await waitFor(() => {
      expect(screen.getByText("В секвенции")).toBeInTheDocument();
    });
  });

  it("maps verify statuses correctly", async () => {
    mockGetProspects.mockResolvedValue([
      makeProspect({ name: "A", email: "a@t.com", verify_status: "valid", verify_score: 95 }),
    ]);

    render(<ProspectsPage />);

    await waitFor(() => {
      expect(screen.getByText(/Валидный/)).toBeInTheDocument();
      expect(screen.getByText(/95/)).toBeInTheDocument();
    });
  });

  it("shows AI analytics hint for unchecked prospects", async () => {
    mockGetProspects.mockResolvedValue([
      makeProspect({ name: "A", email: "a@t.com", verify_status: "not_checked" }),
    ]);

    render(<ProspectsPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/1 проспектов не проверены/),
      ).toBeInTheDocument();
    });
  });
});
