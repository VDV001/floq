import type { ReactNode } from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import type { UserSettings } from "@/lib/api";
import AutomationsPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), back: vi.fn() }),
  usePathname: () => "/automations",
}));

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock("@/components/ui/switch", () => ({
  Switch: ({ checked, onCheckedChange, ...props }: { checked: boolean; onCheckedChange: (v: boolean) => void; [key: string]: unknown }) => (
    <button
      role="switch"
      aria-checked={checked}
      onClick={() => onCheckedChange?.(!checked)}
      {...props}
    />
  ),
}));

const mockSettings = {
  auto_qualify: true,
  auto_draft: true,
  auto_send: false,
  auto_followup: true,
  auto_prospect_to_lead: true,
  auto_verify_import: false,
  auto_send_delay_min: 5,
  auto_followup_days: 2,
};

vi.mock("@/lib/api", () => ({
  api: {
    getSettings: vi.fn(),
    updateSettings: vi.fn(),
  },
}));

import { api } from "@/lib/api";

describe("AutomationsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getSettings).mockResolvedValue(mockSettings as UserSettings);
    vi.mocked(api.updateSettings).mockResolvedValue(mockSettings as UserSettings);
  });

  it("renders page header", async () => {
    render(<AutomationsPage />);

    expect(screen.getByText("Автоматизации")).toBeInTheDocument();
  });

  it("renders all automation cards", async () => {
    render(<AutomationsPage />);

    await waitFor(() => {
      expect(screen.getByText("Авто-квалификация")).toBeInTheDocument();
      expect(screen.getByText("Авто-черновик")).toBeInTheDocument();
      expect(screen.getByText("Авто-отправка email")).toBeInTheDocument();
      expect(screen.getByText("Авто-фоллоуап")).toBeInTheDocument();
      expect(screen.getByText("Проспект → Лид")).toBeInTheDocument();
      expect(screen.getByText("Верификация при импорте")).toBeInTheDocument();
    });
  });

  it("renders quick actions section", async () => {
    render(<AutomationsPage />);

    expect(screen.getByText("Быстрые действия")).toBeInTheDocument();
  });

  it("loads settings from API on mount", async () => {
    render(<AutomationsPage />);

    await waitFor(() => {
      expect(api.getSettings).toHaveBeenCalled();
    });
  });

  it("renders toggle-all button", async () => {
    render(<AutomationsPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Включить все|Выключить все/)
      ).toBeInTheDocument();
    });
  });
});
