import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import SettingsPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), back: vi.fn() }),
  usePathname: () => "/settings",
}));

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: any) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const mockSettings = {
  full_name: "Тест Юзер",
  email: "test@example.com",
  telegram_bot_token: "",
  telegram_bot_active: false,
  imap_host: "",
  imap_port: "",
  imap_user: "",
  imap_password: "",
  resend_api_key: "",
  smtp_host: "",
  smtp_port: "",
  smtp_user: "",
  smtp_password: "",
  smtp_active: false,
  ai_provider: "anthropic",
  ai_model: "claude-sonnet-4-20250514",
  ai_api_key: "sk-test",
  imap_active: false,
  resend_active: false,
  ai_active: true,
  notify_telegram: true,
  notify_email_digest: false,
  auto_qualify: true,
  auto_draft: true,
  auto_send: false,
  auto_send_delay_min: 5,
  auto_followup: true,
  auto_followup_days: 2,
  auto_prospect_to_lead: true,
  auto_verify_import: false,
};

vi.mock("@/lib/api", () => ({
  api: {
    getSettings: vi.fn(),
    updateSettings: vi.fn(),
    testAI: vi.fn(),
    testIMAP: vi.fn(),
    testSMTP: vi.fn(),
    testResend: vi.fn(),
    tgAccountStatus: vi.fn(),
  },
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

import { api } from "@/lib/api";

describe("SettingsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getSettings).mockResolvedValue(mockSettings as any);
    vi.mocked(api.tgAccountStatus).mockResolvedValue({ connected: false, phone: "" });
  });

  it("renders settings page with header after loading", async () => {
    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByText("Настройки")).toBeInTheDocument();
    });
  });

  it("loads settings from API", async () => {
    render(<SettingsPage />);

    await waitFor(() => {
      expect(api.getSettings).toHaveBeenCalled();
    });
  });

  it("renders Telegram section", async () => {
    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByText("Telegram bot")).toBeInTheDocument();
    });
  });
});
