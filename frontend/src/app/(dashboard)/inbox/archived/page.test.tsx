import type { ReactNode } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import { NotificationProvider } from "@/components/notifications/NotificationProvider";
import userEvent from "@testing-library/user-event";
import { vi, describe, it, expect, beforeEach } from "vitest";

/* ---- Mocks ---- */

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), back: vi.fn() }),
  usePathname: () => "/inbox/archived",
}));

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

const mockGetArchivedLeads = vi.fn();
const mockUnarchiveLead = vi.fn();

vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ApiError: actual.ApiError,
    api: {
      getArchivedLeads: (...args: unknown[]) => mockGetArchivedLeads(...args),
      unarchiveLead: (...args: unknown[]) => mockUnarchiveLead(...args),
    },
  };
});

vi.mock("@/lib/utils", () => ({
  cn: (...args: unknown[]) => args.filter(Boolean).join(" "),
}));

import ArchivedLeadsPage from "./page";

/* ---- Helpers ---- */

function makeArchivedLead(overrides: Partial<{
  id: string;
  contact_name: string;
  company: string;
  channel: string;
  status: string;
  source_name: string;
  archived_at: string;
}> = {}) {
  return {
    id: overrides.id ?? "lead-1",
    user_id: "u1",
    contact_name: overrides.contact_name ?? "Иван Петров",
    company: overrides.company ?? "ООО Ромашка",
    channel: overrides.channel ?? "telegram",
    first_message: "Здравствуйте",
    status: overrides.status ?? "qualified",
    source_name: overrides.source_name,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    archived_at: overrides.archived_at ?? new Date().toISOString(),
  };
}

/* ---- Tests ---- */

describe("ArchivedLeadsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders empty state when no archived leads", async () => {
    mockGetArchivedLeads.mockResolvedValue([]);

    render(<ArchivedLeadsPage />, { wrapper: NotificationProvider });

    await waitFor(() => {
      expect(screen.getByText("Архив пуст")).toBeInTheDocument();
    });
  });

  it("renders archived lead rows with company and status", async () => {
    mockGetArchivedLeads.mockResolvedValue([
      makeArchivedLead({ id: "l1", company: "Архив Компания", status: "qualified" }),
    ]);

    render(<ArchivedLeadsPage />, { wrapper: NotificationProvider });

    await waitFor(() => {
      expect(screen.getByText("Архив Компания")).toBeInTheDocument();
    });
    expect(screen.getByText("Квалифицирован")).toBeInTheDocument();
    expect(screen.getByText(/В архиве ·/)).toBeInTheDocument();
  });

  it("row links to the lead detail page", async () => {
    mockGetArchivedLeads.mockResolvedValue([makeArchivedLead({ id: "lead-42" })]);

    render(<ArchivedLeadsPage />, { wrapper: NotificationProvider });

    await waitFor(() => {
      const link = screen
        .getAllByRole("link")
        .find((a) => a.getAttribute("href") === "/inbox/lead-42");
      expect(link).toBeDefined();
    });
  });

  it("unarchives a lead, removes the row and shows a success toast", async () => {
    mockGetArchivedLeads.mockResolvedValue([
      makeArchivedLead({ id: "l1", company: "Вернуть Компания" }),
    ]);
    mockUnarchiveLead.mockResolvedValue(undefined);

    const user = userEvent.setup();
    render(<ArchivedLeadsPage />, { wrapper: NotificationProvider });

    await waitFor(() => {
      expect(screen.getByText("Вернуть Компания")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /Разархивировать/ }));

    await waitFor(() => {
      expect(mockUnarchiveLead).toHaveBeenCalledWith("l1");
    });
    await waitFor(() => {
      expect(screen.queryByText("Вернуть Компания")).not.toBeInTheDocument();
    });
    expect(screen.getByText("Лид возвращён")).toBeInTheDocument();
  });

  it("shows a load-error state (not «Архив пуст») when the fetch fails", async () => {
    mockGetArchivedLeads.mockRejectedValue(new Error("boom"));

    render(<ArchivedLeadsPage />, { wrapper: NotificationProvider });

    await waitFor(() => {
      expect(screen.getByText("Не удалось загрузить архив")).toBeInTheDocument();
    });
    // Must NOT masquerade as an empty archive.
    expect(screen.queryByText("Архив пуст")).not.toBeInTheDocument();
  });

  it("lets another row unarchive while a first unarchive is still in flight", async () => {
    mockGetArchivedLeads.mockResolvedValue([
      makeArchivedLead({ id: "slow", company: "Медленный" }),
      makeArchivedLead({ id: "fast", company: "Быстрый" }),
    ]);
    // First request never resolves; the second resolves normally. A global
    // guard would block the second click — a per-row guard does not.
    let resolveFast: (v?: unknown) => void = () => {};
    mockUnarchiveLead.mockImplementation((id: string) => {
      if (id === "slow") return new Promise(() => {});
      return new Promise((res) => {
        resolveFast = res;
      });
    });

    const user = userEvent.setup();
    render(<ArchivedLeadsPage />, { wrapper: NotificationProvider });

    await waitFor(() => {
      expect(screen.getByText("Медленный")).toBeInTheDocument();
    });

    const buttons = screen.getAllByRole("button", { name: /Разархивировать|Возвращаем/ });
    await user.click(buttons[0]); // slow — hangs
    await user.click(buttons[1]); // fast — must still fire

    await waitFor(() => {
      expect(mockUnarchiveLead).toHaveBeenCalledWith("fast");
    });
    resolveFast();
    await waitFor(() => {
      expect(screen.queryByText("Быстрый")).not.toBeInTheDocument();
    });
    // The hung row is untouched.
    expect(screen.getByText("Медленный")).toBeInTheDocument();
  });

  it("keeps the row and surfaces an error toast when unarchive fails", async () => {
    mockGetArchivedLeads.mockResolvedValue([
      makeArchivedLead({ id: "l1", company: "Остаться Компания" }),
    ]);
    mockUnarchiveLead.mockRejectedValue(new Error("boom"));

    const user = userEvent.setup();
    render(<ArchivedLeadsPage />, { wrapper: NotificationProvider });

    await waitFor(() => {
      expect(screen.getByText("Остаться Компания")).toBeInTheDocument();
    });

    await user.click(screen.getByRole("button", { name: /Разархивировать/ }));

    await waitFor(() => {
      expect(screen.getByText("Не удалось разархивировать лид")).toBeInTheDocument();
    });
    // Row stays — the optimistic removal only happens on success.
    expect(screen.getByText("Остаться Компания")).toBeInTheDocument();
  });
});
