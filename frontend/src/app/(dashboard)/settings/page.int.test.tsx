import { describe, it, expect } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";

import { server, url } from "@/__tests__/integration/server";
import type { OnecConfig, OnecMapping, UserSettings } from "@/lib/api";

import SettingsPage from "./page";

// Integration: real SettingsPage + useSettingsCore/sub-hooks + useOnecSettings +
// lib/api.ts, network via MSW. On mount the page fires four GETs:
//   /api/settings                  -> profile, channels, AI hydration
//   /api/telegram-account/status   -> TG personal account section
//   /api/onec/config               -> 1C config section
//   /api/onec/mapping              -> 1C mapping editor
// Saving the profile button issues PUT /api/settings; the channel/AI "test"
// buttons POST /api/settings/test-* (and AI/SMTP/Resend may follow with a PUT).
const settings: UserSettings = {
  full_name: "Иван Тестов",
  email: "ivan@floq.io",
  telegram_bot_token: "",
  telegram_bot_active: false,
  imap_host: "imap.gmail.com",
  imap_port: "993",
  imap_user: "ivan@floq.io",
  imap_password: "...abcd",
  resend_api_key: "",
  smtp_host: "",
  smtp_port: "",
  smtp_user: "",
  smtp_password: "",
  smtp_active: false,
  ai_provider: "ollama",
  ai_model: "gemma3:4b",
  ai_api_key: "",
  imap_active: true,
  resend_active: false,
  ai_active: false,
  notify_telegram: false,
  notify_email_digest: false,
  auto_qualify: true,
  auto_draft: true,
  auto_send: false,
  auto_send_delay_min: 5,
  auto_followup: true,
  auto_followup_days: 2,
  auto_prospect_to_lead: true,
  auto_verify_import: false,
  aggregated_inbox_view: false,
};

const onecConfig: OnecConfig = {
  base_url: "",
  auth_type: "basic",
  auth_secret: "",
  webhook_secret: "",
  is_active: false,
};

const onecMapping: OnecMapping = { rules: [] };

function mountWith(
  overrides: Partial<UserSettings> = {},
  extra: Parameters<typeof server.use> = [],
) {
  server.use(
    http.get(url("/api/settings"), () => HttpResponse.json({ ...settings, ...overrides })),
    http.get(url("/api/telegram-account/status"), () =>
      HttpResponse.json({ connected: false, phone: "" }),
    ),
    http.get(url("/api/onec/config"), () => HttpResponse.json(onecConfig)),
    http.get(url("/api/onec/mapping"), () => HttpResponse.json(onecMapping)),
    ...extra,
  );
}

describe("settings page (integration)", () => {
  it("loads settings from the API and renders the form fields", async () => {
    mountWith();

    render(<SettingsPage />);

    // Profile card hydrates from /api/settings.
    expect(await screen.findByText("Иван Тестов")).toBeInTheDocument();
    expect(screen.getByText("ivan@floq.io")).toBeInTheDocument();

    // Channel section headings render.
    expect(screen.getByText("Каналы связи")).toBeInTheDocument();
    expect(screen.getByText("Email IMAP")).toBeInTheDocument();
    expect(screen.getByText("ИИ Провайдер")).toBeInTheDocument();

    // IMAP inputs are populated from the fetched settings.
    expect(screen.getByDisplayValue("imap.gmail.com")).toBeInTheDocument();
    expect(screen.getByDisplayValue("993")).toBeInTheDocument();
    // AI provider/model hydrate.
    expect(screen.getByDisplayValue("gemma3:4b")).toBeInTheDocument();

    // 1C section renders once its config/mapping resolve.
    expect(await screen.findByText("Интеграция с 1С")).toBeInTheDocument();
  });

  it("saves the profile through a PUT and shows the success toast", async () => {
    const user = userEvent.setup({ delay: null });
    let putBody: Partial<UserSettings> | null = null;
    mountWith({}, [
      http.put(url("/api/settings"), async ({ request }) => {
        putBody = (await request.json()) as Partial<UserSettings>;
        return HttpResponse.json({ ...settings });
      }),
    ]);

    render(<SettingsPage />);
    await screen.findByText("Иван Тестов");

    // Edit the IMAP host (controlled input), then click the main save button.
    fireEvent.change(screen.getByDisplayValue("imap.gmail.com"), {
      target: { value: "imap.fastmail.com" },
    });
    await user.click(screen.getByRole("button", { name: /Сохранить изменения/ }));

    await waitFor(() => expect(putBody).not.toBeNull());

    // handleSave maps the IMAP fields + AI provider/model into the PUT body.
    expect(putBody).toMatchObject({
      imap_host: "imap.fastmail.com",
      imap_port: "993",
      imap_user: "ivan@floq.io",
      ai_provider: "ollama",
      ai_model: "gemma3:4b",
      notify_telegram: true,
      notify_email_digest: false,
    });

    // Success toast.
    expect(await screen.findByText("Настройки сохранены")).toBeInTheDocument();
  });

  it("runs the IMAP test connection through a POST and shows the result banner", async () => {
    const user = userEvent.setup({ delay: null });
    let testBody: Record<string, unknown> | null = null;
    mountWith({}, [
      http.post(url("/api/settings/test-imap"), async ({ request }) => {
        testBody = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ success: true, message: "IMAP подключён" });
      }),
    ]);

    render(<SettingsPage />);
    await screen.findByText("Email IMAP");

    // The IMAP section's "Тест соединения" button is the first one on the page
    // (the 1C section has an identically labelled button further down).
    const testButtons = screen.getAllByRole("button", { name: "Тест соединения" });
    await user.click(testButtons[0]);

    expect(await screen.findByText("IMAP подключён")).toBeInTheDocument();
    expect(testBody).toMatchObject({
      host: "imap.gmail.com",
      port: "993",
      user: "ivan@floq.io",
      use_stored: true,
    });
  });
});
