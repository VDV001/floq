import { describe, it, expect } from "vitest";
import { render, screen, waitFor, fireEvent, within } from "@testing-library/react";
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

// mountFull registers the four mount GETs but lets each test pick the settings /
// 1C config / TG status payloads directly — avoids handler-precedence games when
// a flow needs a *different* mount response (e.g. an already-connected TG account
// or a populated 1C config) instead of the defaults.
function mountFull(opts: {
  settings?: Partial<UserSettings>;
  onec?: Partial<OnecConfig>;
  tgStatus?: { connected: boolean; phone: string };
  extra?: Parameters<typeof server.use>;
}) {
  server.use(
    http.get(url("/api/settings"), () => HttpResponse.json({ ...settings, ...opts.settings })),
    http.get(url("/api/telegram-account/status"), () =>
      HttpResponse.json(opts.tgStatus ?? { connected: false, phone: "" }),
    ),
    http.get(url("/api/onec/config"), () => HttpResponse.json({ ...onecConfig, ...opts.onec })),
    http.get(url("/api/onec/mapping"), () => HttpResponse.json(onecMapping)),
    ...(opts.extra ?? []),
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

  it("shows the error banner when the profile PUT fails", async () => {
    const user = userEvent.setup({ delay: null });
    mountWith({}, [
      http.put(url("/api/settings"), () =>
        HttpResponse.json({ error: "boom" }, { status: 500 }),
      ),
    ]);

    render(<SettingsPage />);
    await screen.findByText("Иван Тестов");

    await user.click(screen.getByRole("button", { name: /Сохранить изменения/ }));

    // core.save() catches and flips saveResult to "error".
    expect(await screen.findByText("Ошибка сохранения")).toBeInTheDocument();
  });

  it("runs the SMTP test and persists the verified host on success", async () => {
    const user = userEvent.setup({ delay: null });
    let smtpBody: Record<string, unknown> | null = null;
    let putBody: Partial<UserSettings> | null = null;
    mountWith({}, [
      http.post(url("/api/settings/test-smtp"), async ({ request }) => {
        smtpBody = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ success: true, message: "SMTP готов" });
      }),
      http.put(url("/api/settings"), async ({ request }) => {
        putBody = (await request.json()) as Partial<UserSettings>;
        return HttpResponse.json({ ...settings });
      }),
    ]);

    render(<SettingsPage />);
    await screen.findByText("SMTP (отправка писем)");

    // SMTP host/user start empty — fill them so useSmtpSettings.test() actually
    // hits the network (it short-circuits when host or user is blank).
    fireEvent.change(screen.getByPlaceholderText("smtp.mail.ru"), {
      target: { value: "smtp.fastmail.com" },
    });
    fireEvent.change(screen.getByPlaceholderText("hello@yourdomain.com"), {
      target: { value: "ops@floq.io" },
    });

    // "Тест соединения" appears in IMAP[0], SMTP[1] and 1C[2]; pick SMTP.
    const testButtons = screen.getAllByRole("button", { name: "Тест соединения" });
    await user.click(testButtons[1]);

    expect(await screen.findByText("SMTP готов")).toBeInTheDocument();
    expect(smtpBody).toMatchObject({
      host: "smtp.fastmail.com",
      port: "465",
      user: "ops@floq.io",
    });
    // On success the hook flushes the verified host/port/user via PUT.
    await waitFor(() => expect(putBody).not.toBeNull());
    expect(putBody).toMatchObject({
      smtp_host: "smtp.fastmail.com",
      smtp_port: "465",
      smtp_user: "ops@floq.io",
    });
  });

  it("surfaces the Resend test error without persisting", async () => {
    const user = userEvent.setup({ delay: null });
    let putCalled = false;
    mountWith({}, [
      http.post(url("/api/settings/test-resend"), () =>
        HttpResponse.json({ success: false, error: "Resend ключ отклонён" }),
      ),
      http.put(url("/api/settings"), () => {
        putCalled = true;
        return HttpResponse.json({ ...settings });
      }),
    ]);

    render(<SettingsPage />);
    await screen.findByText("Resend API");

    // The Resend "Проверить" button is disabled until a key is typed.
    fireEvent.change(screen.getByPlaceholderText("re_123456789..."), {
      target: { value: "re_secret_key" },
    });
    await user.click(screen.getByRole("button", { name: "Проверить" }));

    expect(await screen.findByText("Resend ключ отклонён")).toBeInTheDocument();
    // A failed test never persists the key.
    expect(putCalled).toBe(false);
  });

  it("runs the AI test and persists provider/model on success", async () => {
    const user = userEvent.setup({ delay: null });
    let aiBody: Record<string, unknown> | null = null;
    let putBody: Partial<UserSettings> | null = null;
    mountWith({}, [
      http.post(url("/api/settings/test-ai"), async ({ request }) => {
        aiBody = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ success: true, message: "ИИ готов" });
      }),
      http.put(url("/api/settings"), async ({ request }) => {
        putBody = (await request.json()) as Partial<UserSettings>;
        return HttpResponse.json({ ...settings });
      }),
    ]);

    render(<SettingsPage />);
    await screen.findByText("ИИ Провайдер");

    await user.click(screen.getByRole("button", { name: "Проверить подключение" }));

    expect(await screen.findByText("ИИ готов")).toBeInTheDocument();
    // Ollama needs no key — use_stored is true.
    expect(aiBody).toMatchObject({
      provider: "ollama",
      model: "gemma3:4b",
      use_stored: true,
    });
    await waitFor(() => expect(putBody).not.toBeNull());
    expect(putBody).toMatchObject({ ai_provider: "ollama", ai_model: "gemma3:4b" });
  });

  it("advances the Telegram account flow after send-code", async () => {
    const user = userEvent.setup({ delay: null });
    let sendBody: Record<string, unknown> | null = null;
    mountWith({}, [
      http.post(url("/api/telegram-account/send-code"), async ({ request }) => {
        sendBody = (await request.json()) as Record<string, unknown>;
        return HttpResponse.json({ code_hash: "hash-xyz" });
      }),
    ]);

    render(<SettingsPage />);
    await screen.findByText("TG аккаунт (рассылка)");

    fireEvent.change(screen.getByPlaceholderText("+7 999 123 4567"), {
      target: { value: "+79991234567" },
    });
    await user.click(screen.getByRole("button", { name: "Отправить код" }));

    // Step advances to "code_sent": the code-entry UI appears.
    expect(await screen.findByText(/Код отправлен на/)).toBeInTheDocument();
    expect(screen.getByPlaceholderText("Код из Telegram")).toBeInTheDocument();
    expect(sendBody).toMatchObject({ phone: "+79991234567" });
  });

  it("shows a friendly error when send-code is rejected", async () => {
    const user = userEvent.setup({ delay: null });
    mountWith({}, [
      http.post(url("/api/telegram-account/send-code"), () =>
        HttpResponse.json({ error: "PHONE_NUMBER_INVALID" }, { status: 400 }),
      ),
    ]);

    render(<SettingsPage />);
    await screen.findByText("TG аккаунт (рассылка)");

    fireEvent.change(screen.getByPlaceholderText("+7 999 123 4567"), {
      target: { value: "+79990000000" },
    });
    await user.click(screen.getByRole("button", { name: "Отправить код" }));

    expect(
      await screen.findByText("Неверный номер телефона. Проверьте формат."),
    ).toBeInTheDocument();
  });

  it("disconnects an already-connected Telegram account", async () => {
    const user = userEvent.setup({ delay: null });
    let deleted = false;
    mountFull({
      tgStatus: { connected: true, phone: "+79995551122" },
      extra: [
        http.delete(url("/api/telegram-account"), () => {
          deleted = true;
          return new HttpResponse(null, { status: 204 });
        }),
      ],
    });

    render(<SettingsPage />);
    // The status GET hydrates the connected view.
    expect(await screen.findByText("+79995551122")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Отключить" }));

    await waitFor(() => expect(deleted).toBe(true));
    // Back to the idle phone-entry step.
    expect(await screen.findByPlaceholderText("+7 999 123 4567")).toBeInTheDocument();
  });

  it("saves the 1C config through PUT /api/onec/config", async () => {
    const user = userEvent.setup({ delay: null });
    let onecBody: Record<string, unknown> | null = null;
    mountFull({
      onec: { base_url: "https://1c.old/odata", is_active: false },
      extra: [
        http.put(url("/api/onec/config"), async ({ request }) => {
          onecBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json({
            ...onecConfig,
            base_url: "https://1c.new/odata",
            is_active: true,
          });
        }),
      ],
    });

    render(<SettingsPage />);
    await screen.findByText("Интеграция с 1С");

    fireEvent.change(screen.getByLabelText("Адрес OData-сервиса 1С"), {
      target: { value: "https://1c.new/odata" },
    });
    await user.click(screen.getByRole("checkbox", { name: "Включить интеграцию" }));
    // The 1C section's own save button is labelled exactly "Сохранить".
    await user.click(screen.getByRole("button", { name: "Сохранить" }));

    await waitFor(() => expect(onecBody).not.toBeNull());
    expect(onecBody).toMatchObject({
      base_url: "https://1c.new/odata",
      auth_type: "basic",
      is_active: true,
    });
    // No secret typed -> auth_secret omitted so the stored one is kept.
    expect(onecBody).not.toHaveProperty("auth_secret");
  });

  it("runs the 1C connection test", async () => {
    const user = userEvent.setup({ delay: null });
    let testBody: Record<string, unknown> | null = null;
    mountFull({
      onec: { base_url: "https://1c.example/odata", is_active: true },
      extra: [
        http.post(url("/api/onec/test"), async ({ request }) => {
          testBody = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json({ success: true });
        }),
      ],
    });

    render(<SettingsPage />);
    await screen.findByText("Интеграция с 1С");

    // "Тест соединения" buttons: IMAP[0], SMTP[1], 1C[2].
    const testButtons = screen.getAllByRole("button", { name: "Тест соединения" });
    await user.click(testButtons[2]);

    expect(await screen.findByText("Соединение с 1С установлено")).toBeInTheDocument();
    expect(testBody).toMatchObject({
      base_url: "https://1c.example/odata",
      auth_type: "basic",
    });
  });

  it("regenerates the 1C webhook secret and reveals it once", async () => {
    const user = userEvent.setup({ delay: null });
    mountFull({
      onec: { base_url: "https://1c.example/odata", is_active: true },
      extra: [
        http.post(url("/api/onec/config/regenerate-webhook"), () =>
          HttpResponse.json({ webhook_secret: "whsec_brand_new_123" }),
        ),
      ],
    });

    render(<SettingsPage />);
    await screen.findByText("Интеграция с 1С");

    await user.click(screen.getByRole("button", { name: /Сгенерировать заново/ }));

    // The freshly minted secret is surfaced once in the amber callout.
    expect(await screen.findByText("whsec_brand_new_123")).toBeInTheDocument();
  });

  it("persists the aggregated-inbox toggle via the settings PUT", async () => {
    const user = userEvent.setup({ delay: null });
    let putBody: Partial<UserSettings> | null = null;
    mountWith({ aggregated_inbox_view: false }, [
      http.put(url("/api/settings"), async ({ request }) => {
        putBody = (await request.json()) as Partial<UserSettings>;
        return HttpResponse.json({ ...settings, aggregated_inbox_view: true });
      }),
    ]);

    render(<SettingsPage />);
    await screen.findByText("Иван Тестов");

    const region = screen.getByRole("region", { name: "Вид входящих" });
    await user.click(within(region).getByRole("checkbox"));

    await waitFor(() => expect(putBody).not.toBeNull());
    expect(putBody).toMatchObject({ aggregated_inbox_view: true });
    // core.save() also raises the global success toast.
    expect(await screen.findByText("Настройки сохранены")).toBeInTheDocument();
  });
});
