import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";

vi.mock("@/lib/api", () => ({
  api: {
    getSettings: vi.fn(),
    updateSettings: vi.fn(),
    tgAccountStatus: vi.fn(),
    tgAccountSendCode: vi.fn(),
    tgAccountVerify: vi.fn(),
    tgAccountDisconnect: vi.fn(),
    testIMAP: vi.fn(),
    testResend: vi.fn(),
    testSMTP: vi.fn(),
    testAI: vi.fn(),
  },
}));

import { api, type UserSettings } from "@/lib/api";
import {
  useSettingsCore,
  useTelegramBot,
  useTelegramAccount,
  useImapSettings,
  useResendSettings,
  useSmtpSettings,
  useAiSettings,
  PROVIDER_DEFAULTS,
} from "./useSettings";

const mockedApi = vi.mocked(api);

function makeSettings(overrides: Partial<UserSettings> = {}): UserSettings {
  return {
    full_name: "Test User",
    email: "test@example.com",
    telegram_bot_token: "",
    telegram_bot_active: false,
    imap_host: "imap.example.com",
    imap_port: "993",
    imap_user: "user@example.com",
    imap_password: "...masked",
    resend_api_key: "",
    smtp_host: "smtp.example.com",
    smtp_port: "465",
    smtp_user: "user@example.com",
    smtp_password: "...masked",
    smtp_active: false,
    ai_provider: "ollama",
    ai_model: "gemma3:4b",
    ai_api_key: "",
    imap_active: true,
    resend_active: false,
    ai_active: false,
    notify_telegram: false,
    notify_email_digest: false,
    auto_qualify: false,
    auto_draft: false,
    auto_send: false,
    auto_send_delay_min: 5,
    auto_followup: false,
    auto_followup_days: 2,
    auto_prospect_to_lead: false,
    auto_verify_import: false,
    ...overrides,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
});

// ─── useSettingsCore ─────────────────────────────────────────

describe("useSettingsCore", () => {
  it("loads settings on mount", async () => {
    const settings = makeSettings();
    mockedApi.getSettings.mockResolvedValue(settings);

    const { result } = renderHook(() => useSettingsCore());

    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.settings).toEqual(settings);
  });

  it("sets loading=false even on error", async () => {
    mockedApi.getSettings.mockRejectedValue(new Error("fail"));

    const { result } = renderHook(() => useSettingsCore());

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.settings).toBeNull();
  });

  it("save updates settings and shows success", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const initial = makeSettings();
    const updated = makeSettings({ full_name: "Updated" });
    mockedApi.getSettings.mockResolvedValue(initial);
    mockedApi.updateSettings.mockResolvedValue(updated);

    const { result } = renderHook(() => useSettingsCore());
    await waitFor(() => expect(result.current.loading).toBe(false));

    await act(async () => {
      await result.current.save({ full_name: "Updated" });
    });

    expect(result.current.settings?.full_name).toBe("Updated");
    expect(result.current.saveResult).toBe("success");

    act(() => { vi.advanceTimersByTime(4100); });
    expect(result.current.saveResult).toBeNull();
    vi.useRealTimers();
  });

  it("save shows error on failure", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mockedApi.getSettings.mockResolvedValue(makeSettings());
    mockedApi.updateSettings.mockRejectedValue(new Error("fail"));

    const { result } = renderHook(() => useSettingsCore());
    await waitFor(() => expect(result.current.loading).toBe(false));

    await act(async () => {
      await result.current.save({ full_name: "x" });
    });

    expect(result.current.saveResult).toBe("error");

    act(() => { vi.advanceTimersByTime(4100); });
    expect(result.current.saveResult).toBeNull();
    vi.useRealTimers();
  });
});

// ─── useTelegramBot ──────────────────────────────────────────

describe("useTelegramBot", () => {
  it("connects with valid token", async () => {
    const updated = makeSettings({ telegram_bot_token: "...abc123" });
    mockedApi.updateSettings.mockResolvedValue(updated);

    const settings = makeSettings();
    const setSettings = vi.fn();

    const { result } = renderHook(() => useTelegramBot(settings, setSettings));

    act(() => { result.current.setTgToken("123:ABC"); });

    await act(async () => { await result.current.connect(); });

    expect(mockedApi.updateSettings).toHaveBeenCalledWith({ telegram_bot_token: "123:ABC" });
    expect(setSettings).toHaveBeenCalledWith(updated);
    expect(result.current.tgToken).toBe("");
  });

  it("does not connect if token starts with '...'", async () => {
    const { result } = renderHook(() => useTelegramBot(makeSettings(), vi.fn()));

    act(() => { result.current.setTgToken("...masked"); });

    await act(async () => { await result.current.connect(); });

    expect(mockedApi.updateSettings).not.toHaveBeenCalled();
  });

  it("does not connect if token is empty", async () => {
    const { result } = renderHook(() => useTelegramBot(makeSettings(), vi.fn()));

    await act(async () => { await result.current.connect(); });

    expect(mockedApi.updateSettings).not.toHaveBeenCalled();
  });

  it("returns maskedToken and botActive from settings", () => {
    const settings = makeSettings({ telegram_bot_token: "...tok", telegram_bot_active: true });
    const { result } = renderHook(() => useTelegramBot(settings, vi.fn()));

    expect(result.current.maskedToken).toBe("...tok");
    expect(result.current.botActive).toBe(true);
  });
});

// ─── useTelegramAccount ──────────────────────────────────────

describe("useTelegramAccount", () => {
  it("checks status on mount — connected", async () => {
    mockedApi.tgAccountStatus.mockResolvedValue({ connected: true, phone: "+79001234567" });

    const { result } = renderHook(() => useTelegramAccount());

    await waitFor(() => expect(result.current.step).toBe("connected"));
    expect(result.current.connectedPhone).toBe("+79001234567");
  });

  it("checks status on mount — not connected", async () => {
    mockedApi.tgAccountStatus.mockResolvedValue({ connected: false, phone: "" });

    const { result } = renderHook(() => useTelegramAccount());

    await waitFor(() => expect(result.current.step).toBe("idle"));
  });

  const invalidPhones = [
    { phone: "1234", desc: "too short" },
    { phone: "notaphone", desc: "no +" },
  ];

  it.each(invalidPhones)("sendCode rejects invalid phone ($desc)", async ({ phone }) => {
    mockedApi.tgAccountStatus.mockResolvedValue({ connected: false, phone: "" });

    const { result } = renderHook(() => useTelegramAccount());
    await waitFor(() => expect(result.current.step).toBe("idle"));

    act(() => { result.current.setPhone(phone); });

    await act(async () => { await result.current.sendCode(); });

    expect(result.current.error).toContain("формате");
    expect(mockedApi.tgAccountSendCode).not.toHaveBeenCalled();
  });

  it("sendCode succeeds and transitions to code_sent", async () => {
    mockedApi.tgAccountStatus.mockResolvedValue({ connected: false, phone: "" });
    mockedApi.tgAccountSendCode.mockResolvedValue({ code_hash: "hash123" });

    const { result } = renderHook(() => useTelegramAccount());
    await waitFor(() => expect(result.current.step).toBe("idle"));

    act(() => { result.current.setPhone("+79001234567"); });

    await act(async () => { await result.current.sendCode(); });

    expect(result.current.step).toBe("code_sent");
  });

  const verifyErrors = [
    { msg: "PHONE_CODE_INVALID", expected: "Неверный код" },
    { msg: "PHONE_CODE_EXPIRED", expected: "Код истёк" },
    { msg: "SESSION_PASSWORD_NEEDED", expected: "2FA" },
    { msg: "unknown error", expected: "Ошибка авторизации" },
  ];

  it.each(verifyErrors)("verify handles $msg", async ({ msg, expected }) => {
    mockedApi.tgAccountStatus.mockResolvedValue({ connected: false, phone: "" });
    mockedApi.tgAccountVerify.mockRejectedValue(new Error(msg));

    const { result } = renderHook(() => useTelegramAccount());
    await waitFor(() => expect(result.current.step).toBe("idle"));

    await act(async () => { await result.current.verify(); });

    expect(result.current.error).toContain(expected);
  });

  it("verify succeeds and transitions to connected", async () => {
    mockedApi.tgAccountStatus.mockResolvedValue({ connected: false, phone: "" });
    mockedApi.tgAccountVerify.mockResolvedValue({ status: "ok" });

    const { result } = renderHook(() => useTelegramAccount());
    await waitFor(() => expect(result.current.step).toBe("idle"));

    act(() => { result.current.setPhone("+79001234567"); });
    act(() => { result.current.setCode("12345"); });

    await act(async () => { await result.current.verify(); });

    expect(result.current.step).toBe("connected");
    expect(result.current.connectedPhone).toBe("+79001234567");
  });

  it("disconnect resets to idle", async () => {
    mockedApi.tgAccountStatus.mockResolvedValue({ connected: true, phone: "+79001234567" });
    mockedApi.tgAccountDisconnect.mockResolvedValue(undefined);

    const { result } = renderHook(() => useTelegramAccount());
    await waitFor(() => expect(result.current.step).toBe("connected"));

    await act(async () => { await result.current.disconnect(); });

    expect(result.current.step).toBe("idle");
    expect(result.current.connectedPhone).toBe("");
  });

  it("reset clears code and error", async () => {
    mockedApi.tgAccountStatus.mockResolvedValue({ connected: false, phone: "" });

    const { result } = renderHook(() => useTelegramAccount());
    await waitFor(() => expect(result.current.step).toBe("idle"));

    act(() => { result.current.setCode("123"); });
    act(() => { result.current.setError("some error"); });
    act(() => { result.current.reset(); });

    expect(result.current.code).toBe("");
    expect(result.current.error).toBe("");
    expect(result.current.step).toBe("idle");
  });
});

// ─── useImapSettings ─────────────────────────────────────────

describe("useImapSettings", () => {
  it("syncs fields from settings on mount", async () => {
    const settings = makeSettings({ imap_host: "mail.test.com", imap_port: "143", imap_user: "me@test.com" });

    const { result } = renderHook(() => useImapSettings(settings));

    await waitFor(() => {
      expect(result.current.host).toBe("mail.test.com");
      expect(result.current.port).toBe("143");
      expect(result.current.user).toBe("me@test.com");
    });
  });

  it("test requires host and user", async () => {
    const { result } = renderHook(() => useImapSettings(null));

    await act(async () => { await result.current.test(); });

    expect(result.current.testResult?.success).toBe(false);
    expect(result.current.testResult?.error).toContain("хост");
    expect(mockedApi.testIMAP).not.toHaveBeenCalled();
  });

  it("test calls API and sets verified on success", async () => {
    const settings = makeSettings();
    mockedApi.testIMAP.mockResolvedValue({ success: true, message: "OK" });

    const { result } = renderHook(() => useImapSettings(settings));
    await waitFor(() => expect(result.current.host).toBe("imap.example.com"));

    await act(async () => { await result.current.test(); });

    expect(mockedApi.testIMAP).toHaveBeenCalled();
    expect(result.current.testResult?.success).toBe(true);
    expect(result.current.active).toBe(true);
  });

  it("test sets verified=false on error", async () => {
    const settings = makeSettings();
    mockedApi.testIMAP.mockRejectedValue(new Error("fail"));

    const { result } = renderHook(() => useImapSettings(settings));
    await waitFor(() => expect(result.current.host).toBe("imap.example.com"));

    await act(async () => { await result.current.test(); });

    expect(result.current.testResult?.success).toBe(false);
    expect(result.current.active).toBe(false);
  });

  it("active falls back to settings.imap_active when not tested", () => {
    const settings = makeSettings({ imap_active: true });
    const { result } = renderHook(() => useImapSettings(settings));
    expect(result.current.active).toBe(true);
  });
});

// ─── useResendSettings ───────────────────────────────────────

describe("useResendSettings", () => {
  it("test saves key on success", async () => {
    const settings = makeSettings();
    const setSettings = vi.fn();
    const updated = makeSettings({ resend_api_key: "...re_key" });
    mockedApi.testResend.mockResolvedValue({ success: true });
    mockedApi.updateSettings.mockResolvedValue(updated);

    const { result } = renderHook(() => useResendSettings(settings, setSettings));

    act(() => { result.current.setKey("re_1234567890"); });

    await act(async () => { await result.current.test(); });

    expect(mockedApi.testResend).toHaveBeenCalledWith("re_1234567890", false);
    expect(mockedApi.updateSettings).toHaveBeenCalledWith({ resend_api_key: "re_1234567890" });
    expect(setSettings).toHaveBeenCalledWith(updated);
    expect(result.current.key).toBe("");
  });

  it("test uses stored key when key starts with '...'", async () => {
    mockedApi.testResend.mockResolvedValue({ success: true });

    const { result } = renderHook(() => useResendSettings(makeSettings(), vi.fn()));

    act(() => { result.current.setKey("...masked"); });

    await act(async () => { await result.current.test(); });

    expect(mockedApi.testResend).toHaveBeenCalledWith("", true);
  });

  it("active derives from verified or settings", () => {
    const settings = makeSettings({ resend_active: true });
    const { result } = renderHook(() => useResendSettings(settings, vi.fn()));
    expect(result.current.active).toBe(true);
  });
});

// ─── useSmtpSettings ─────────────────────────────────────────

describe("useSmtpSettings", () => {
  it("syncs fields from settings", async () => {
    const settings = makeSettings({ smtp_host: "smtp.test.com", smtp_port: "587", smtp_user: "me@test.com" });

    const { result } = renderHook(() => useSmtpSettings(settings, vi.fn()));

    await waitFor(() => {
      expect(result.current.host).toBe("smtp.test.com");
      expect(result.current.port).toBe("587");
      expect(result.current.user).toBe("me@test.com");
    });
  });

  it("test requires host and user", async () => {
    const { result } = renderHook(() => useSmtpSettings(null, vi.fn()));

    await act(async () => { await result.current.test(); });

    expect(result.current.testResult?.success).toBe(false);
    expect(mockedApi.testSMTP).not.toHaveBeenCalled();
  });

  it("test saves settings on success with new credentials", async () => {
    const settings = makeSettings();
    const setSettings = vi.fn();
    const updated = makeSettings({ smtp_user: "new@test.com" });
    mockedApi.testSMTP.mockResolvedValue({ success: true });
    mockedApi.updateSettings.mockResolvedValue(updated);

    const { result } = renderHook(() => useSmtpSettings(settings, setSettings));
    await waitFor(() => expect(result.current.host).toBe("smtp.example.com"));

    act(() => { result.current.setPassword("newpass"); });

    await act(async () => { await result.current.test(); });

    expect(mockedApi.testSMTP).toHaveBeenCalled();
    expect(mockedApi.updateSettings).toHaveBeenCalled();
    expect(setSettings).toHaveBeenCalledWith(updated);
  });

  it("test sets error on failure", async () => {
    const settings = makeSettings();
    mockedApi.testSMTP.mockRejectedValue(new Error("fail"));

    const { result } = renderHook(() => useSmtpSettings(settings, vi.fn()));
    await waitFor(() => expect(result.current.host).toBe("smtp.example.com"));

    await act(async () => { await result.current.test(); });

    expect(result.current.testResult?.success).toBe(false);
    expect(result.current.active).toBe(false);
  });
});

// ─── useAiSettings ───────────────────────────────────────────

describe("useAiSettings", () => {
  it("syncs provider and model from settings", async () => {
    const settings = makeSettings({ ai_provider: "claude", ai_model: "claude-sonnet-4-20250514" });

    const { result } = renderHook(() => useAiSettings(settings, vi.fn()));

    await waitFor(() => {
      expect(result.current.provider).toBe("claude");
      expect(result.current.model).toBe("claude-sonnet-4-20250514");
    });
  });

  it("test saves provider+model on success", async () => {
    const settings = makeSettings();
    const setSettings = vi.fn();
    const updated = makeSettings({ ai_provider: "openai", ai_model: "gpt-4o" });
    mockedApi.testAI.mockResolvedValue({ success: true });
    mockedApi.updateSettings.mockResolvedValue(updated);

    const { result } = renderHook(() => useAiSettings(settings, setSettings));
    await waitFor(() => expect(result.current.provider).toBe("ollama"));

    act(() => { result.current.setProvider("openai"); });
    act(() => { result.current.setModel("gpt-4o"); });
    act(() => { result.current.setApiKey("sk-123"); });

    await act(async () => { await result.current.test(); });

    expect(mockedApi.testAI).toHaveBeenCalledWith("openai", "gpt-4o", "sk-123", false);
    expect(mockedApi.updateSettings).toHaveBeenCalledWith({
      ai_provider: "openai",
      ai_model: "gpt-4o",
      ai_api_key: "sk-123",
    });
    expect(setSettings).toHaveBeenCalledWith(updated);
    expect(result.current.apiKey).toBe("");
  });

  it("test uses stored key when apiKey starts with '...'", async () => {
    mockedApi.testAI.mockResolvedValue({ success: true });
    mockedApi.updateSettings.mockResolvedValue(makeSettings());

    const { result } = renderHook(() => useAiSettings(makeSettings(), vi.fn()));

    act(() => { result.current.setApiKey("...masked"); });

    await act(async () => { await result.current.test(); });

    expect(mockedApi.testAI).toHaveBeenCalledWith("ollama", "gemma3:4b", "", true);
  });

  it("test sets error on failure", async () => {
    mockedApi.testAI.mockRejectedValue(new Error("fail"));

    const { result } = renderHook(() => useAiSettings(makeSettings(), vi.fn()));

    await act(async () => { await result.current.test(); });

    expect(result.current.testResult?.success).toBe(false);
    expect(result.current.active).toBe(false);
  });

  it("PROVIDER_DEFAULTS has expected keys", () => {
    expect(PROVIDER_DEFAULTS).toHaveProperty("ollama");
    expect(PROVIDER_DEFAULTS).toHaveProperty("claude");
    expect(PROVIDER_DEFAULTS).toHaveProperty("openai");
    expect(PROVIDER_DEFAULTS).toHaveProperty("groq");
  });
});
