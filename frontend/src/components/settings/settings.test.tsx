import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ProfileCard } from "./ProfileCard";
import { NotificationsCard } from "./NotificationsCard";
import { StatusBanner } from "./StatusBanner";
import { ConnectionBadge } from "./ConnectionBadge";
import { HintIcon } from "./HintIcon";
import { TelegramBotSection } from "./TelegramBotSection";
import { ImapSection } from "./ImapSection";
import { ResendSection } from "./ResendSection";
import { SmtpSection } from "./SmtpSection";
import { AiProviderSection } from "./AiProviderSection";
import { TelegramAccountSection } from "./TelegramAccountSection";
import type { UserSettings } from "@/lib/api";

vi.mock("@/components/ui/switch", () => ({
  Switch: ({ checked, onCheckedChange, disabled }: { checked: boolean; onCheckedChange: (v: boolean) => void; disabled?: boolean }) => (
    <button data-testid="switch" onClick={() => !disabled && onCheckedChange(!checked)} aria-disabled={disabled}>
      {checked ? "on" : "off"}
    </button>
  ),
}));

function makeSettings(overrides: Partial<UserSettings> = {}): UserSettings {
  return {
    full_name: "Даниил Тест",
    email: "daniil@test.com",
    telegram_bot_token: "",
    telegram_bot_active: false,
    imap_host: "", imap_port: "993", imap_user: "", imap_password: "",
    resend_api_key: "",
    smtp_host: "", smtp_port: "465", smtp_user: "", smtp_password: "",
    smtp_active: false,
    ai_provider: "ollama", ai_model: "gemma3:4b", ai_api_key: "",
    imap_active: false, resend_active: false, ai_active: false,
    notify_telegram: false, notify_email_digest: false,
    auto_qualify: false, auto_draft: false, auto_send: false,
    auto_send_delay_min: 5, auto_followup: false, auto_followup_days: 2,
    auto_prospect_to_lead: false, auto_verify_import: false,
    ...overrides,
  };
}

// ─── ProfileCard ─────────────────────────────────────────────

describe("ProfileCard", () => {
  it("renders name, email, and initials", () => {
    render(<ProfileCard settings={makeSettings()} />);
    expect(screen.getByText("Даниил Тест")).toBeInTheDocument();
    expect(screen.getByText("daniil@test.com")).toBeInTheDocument();
    expect(screen.getByText("ДТ")).toBeInTheDocument();
  });

  it("shows placeholder when settings is null", () => {
    render(<ProfileCard settings={null} />);
    expect(screen.getByText("??")).toBeInTheDocument();
    expect(screen.getAllByText("—")).toHaveLength(2);
  });

  it("renders change password button", () => {
    render(<ProfileCard settings={makeSettings()} />);
    expect(screen.getByText("Сменить пароль")).toBeInTheDocument();
  });
});

// ─── NotificationsCard ───────────────────────────────────────

describe("NotificationsCard", () => {
  it("renders notification options", () => {
    render(
      <NotificationsCard notifyTg={false} setNotifyTg={vi.fn()} notifyEmail={false} setNotifyEmail={vi.fn()} telegramBotActive={false} />
    );
    expect(screen.getByText("Уведомления")).toBeInTheDocument();
    expect(screen.getByText(/В Telegram/)).toBeInTheDocument();
    expect(screen.getByText(/Еженедельный отчет/)).toBeInTheDocument();
  });

  it("shows bot not connected hint", () => {
    render(
      <NotificationsCard notifyTg={false} setNotifyTg={vi.fn()} notifyEmail={false} setNotifyEmail={vi.fn()} telegramBotActive={false} />
    );
    expect(screen.getByText(/Сначала подключите/)).toBeInTheDocument();
  });

  it("shows bot connected hint", () => {
    render(
      <NotificationsCard notifyTg={false} setNotifyTg={vi.fn()} notifyEmail={false} setNotifyEmail={vi.fn()} telegramBotActive={true} />
    );
    expect(screen.getByText(/Бот подключен/)).toBeInTheDocument();
  });
});

// ─── StatusBanner ────────────────────────────────────────────

describe("StatusBanner", () => {
  it("renders nothing when result is null", () => {
    const { container } = render(<StatusBanner result={null} onDismiss={vi.fn()} />);
    expect(container.firstChild).toBeNull();
  });

  it("renders success message", () => {
    render(<StatusBanner result={{ success: true, message: "OK!" }} onDismiss={vi.fn()} />);
    expect(screen.getByText("OK!")).toBeInTheDocument();
  });

  it("renders error message", () => {
    render(<StatusBanner result={{ success: false, error: "Fail!" }} onDismiss={vi.fn()} />);
    expect(screen.getByText("Fail!")).toBeInTheDocument();
  });

  it("calls onDismiss on close click", async () => {
    const onDismiss = vi.fn();
    render(<StatusBanner result={{ success: true, message: "OK" }} onDismiss={onDismiss} />);

    const buttons = screen.getAllByRole("button");
    await userEvent.click(buttons[0]);
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });
});

// ─── ConnectionBadge ─────────────────────────────────────────

describe("ConnectionBadge", () => {
  it("renders connected state", () => {
    render(<ConnectionBadge active={true} />);
    expect(screen.getByText("Подключен")).toBeInTheDocument();
  });

  it("renders disconnected state", () => {
    render(<ConnectionBadge active={false} />);
    expect(screen.getByText("Не подключен")).toBeInTheDocument();
  });
});

// ─── HintIcon ────────────────────────────────────────────────

describe("HintIcon", () => {
  it("renders hint text", () => {
    render(<HintIcon text="Подсказка тут" />);
    expect(screen.getByText("Подсказка тут")).toBeInTheDocument();
  });
});

// ─── TelegramBotSection ──────────────────────────────────────

describe("TelegramBotSection", () => {
  it("renders input and connect button", () => {
    render(
      <TelegramBotSection botActive={false} maskedToken="" tgToken="" setTgToken={vi.fn()} saving={false} onConnect={vi.fn()} />
    );
    expect(screen.getByText("Telegram bot")).toBeInTheDocument();
    expect(screen.getByText("Подключить")).toBeInTheDocument();
  });

  it("disables button when token is empty", () => {
    render(
      <TelegramBotSection botActive={false} maskedToken="" tgToken="" setTgToken={vi.fn()} saving={false} onConnect={vi.fn()} />
    );
    expect(screen.getByText("Подключить")).toBeDisabled();
  });

  it("calls onConnect when button clicked with valid token", async () => {
    const onConnect = vi.fn();
    render(
      <TelegramBotSection botActive={false} maskedToken="" tgToken="123:ABC" setTgToken={vi.fn()} saving={false} onConnect={onConnect} />
    );
    await userEvent.click(screen.getByText("Подключить"));
    expect(onConnect).toHaveBeenCalledTimes(1);
  });

  it("shows saving state", () => {
    render(
      <TelegramBotSection botActive={false} maskedToken="" tgToken="tok" setTgToken={vi.fn()} saving={true} onConnect={vi.fn()} />
    );
    expect(screen.getByText("...")).toBeInTheDocument();
  });
});

// ─── ImapSection ─────────────────────────────────────────────

describe("ImapSection", () => {
  it("renders all IMAP fields", () => {
    render(
      <ImapSection
        imapHost="imap.test.com" setImapHost={vi.fn()}
        imapPort="993" setImapPort={vi.fn()}
        imapUser="user@test.com" setImapUser={vi.fn()}
        imapPassword="" setImapPassword={vi.fn()}
        maskedPassword="...xxx" active={false} testing={false}
        testResult={null} setTestResult={vi.fn()} onTest={vi.fn()}
      />
    );
    expect(screen.getByText("Email IMAP")).toBeInTheDocument();
    expect(screen.getByDisplayValue("imap.test.com")).toBeInTheDocument();
    expect(screen.getByDisplayValue("993")).toBeInTheDocument();
    expect(screen.getByDisplayValue("user@test.com")).toBeInTheDocument();
  });

  it("calls onTest on button click", async () => {
    const onTest = vi.fn();
    render(
      <ImapSection
        imapHost="" setImapHost={vi.fn()} imapPort="993" setImapPort={vi.fn()}
        imapUser="" setImapUser={vi.fn()} imapPassword="" setImapPassword={vi.fn()}
        maskedPassword="" active={false} testing={false}
        testResult={null} setTestResult={vi.fn()} onTest={onTest}
      />
    );
    await userEvent.click(screen.getByText("Тест соединения"));
    expect(onTest).toHaveBeenCalledTimes(1);
  });

  it("shows testing state", () => {
    render(
      <ImapSection
        imapHost="" setImapHost={vi.fn()} imapPort="" setImapPort={vi.fn()}
        imapUser="" setImapUser={vi.fn()} imapPassword="" setImapPassword={vi.fn()}
        maskedPassword="" active={false} testing={true}
        testResult={null} setTestResult={vi.fn()} onTest={vi.fn()}
      />
    );
    expect(screen.getByText("Проверяем...")).toBeInTheDocument();
  });
});

// ─── ResendSection ───────────────────────────────────────────

describe("ResendSection", () => {
  it("renders Resend API section", () => {
    render(
      <ResendSection
        maskedKey="" resendKey="" setResendKey={vi.fn()}
        active={false} testing={false} testResult={null}
        setTestResult={vi.fn()} hasKey={false} onTest={vi.fn()}
      />
    );
    expect(screen.getByText("Resend API")).toBeInTheDocument();
    expect(screen.getByText("Проверить")).toBeInTheDocument();
  });

  it("disables button when no key and no stored key", () => {
    render(
      <ResendSection
        maskedKey="" resendKey="" setResendKey={vi.fn()}
        active={false} testing={false} testResult={null}
        setTestResult={vi.fn()} hasKey={false} onTest={vi.fn()}
      />
    );
    expect(screen.getByText("Проверить")).toBeDisabled();
  });

  it("enables button when hasKey is true", () => {
    render(
      <ResendSection
        maskedKey="...key" resendKey="" setResendKey={vi.fn()}
        active={false} testing={false} testResult={null}
        setTestResult={vi.fn()} hasKey={true} onTest={vi.fn()}
      />
    );
    expect(screen.getByText("Проверить")).not.toBeDisabled();
  });
});

// ─── SmtpSection ─────────────────────────────────────────────

describe("SmtpSection", () => {
  it("renders all SMTP fields", () => {
    render(
      <SmtpSection
        smtpHost="smtp.mail.ru" setSmtpHost={vi.fn()}
        smtpPort="465" setSmtpPort={vi.fn()}
        smtpUser="me@mail.ru" setSmtpUser={vi.fn()}
        smtpPassword="" setSmtpPassword={vi.fn()}
        maskedPassword="...xxx" active={false} testing={false}
        testResult={null} setTestResult={vi.fn()} onTest={vi.fn()}
      />
    );
    expect(screen.getByText("SMTP (отправка писем)")).toBeInTheDocument();
    expect(screen.getByDisplayValue("smtp.mail.ru")).toBeInTheDocument();
    expect(screen.getByDisplayValue("465")).toBeInTheDocument();
    expect(screen.getByDisplayValue("me@mail.ru")).toBeInTheDocument();
  });

  it("calls onTest on button click", async () => {
    const onTest = vi.fn();
    render(
      <SmtpSection
        smtpHost="" setSmtpHost={vi.fn()} smtpPort="465" setSmtpPort={vi.fn()}
        smtpUser="" setSmtpUser={vi.fn()} smtpPassword="" setSmtpPassword={vi.fn()}
        maskedPassword="" active={false} testing={false}
        testResult={null} setTestResult={vi.fn()} onTest={onTest}
      />
    );
    await userEvent.click(screen.getByText("Тест соединения"));
    expect(onTest).toHaveBeenCalledTimes(1);
  });

  it("shows testing state", () => {
    render(
      <SmtpSection
        smtpHost="" setSmtpHost={vi.fn()} smtpPort="" setSmtpPort={vi.fn()}
        smtpUser="" setSmtpUser={vi.fn()} smtpPassword="" setSmtpPassword={vi.fn()}
        maskedPassword="" active={false} testing={true}
        testResult={null} setTestResult={vi.fn()} onTest={vi.fn()}
      />
    );
    expect(screen.getByText("Проверяем...")).toBeInTheDocument();
  });
});

// ─── AiProviderSection ───────────────────────────────────────

describe("AiProviderSection", () => {
  const defaults: Record<string, string> = {
    ollama: "gemma3:4b",
    claude: "claude-sonnet-4-20250514",
    openai: "gpt-4o",
  };

  it("renders provider and model fields", () => {
    render(
      <AiProviderSection
        aiProvider="ollama" setAiProvider={vi.fn()}
        aiModel="gemma3:4b" setAiModel={vi.fn()}
        aiApiKey="" setAiApiKey={vi.fn()}
        maskedKey="" showApiKey={false} setShowApiKey={vi.fn()}
        active={false} testing={false} testResult={null}
        setTestResult={vi.fn()} hasKey={false} providerDefaults={defaults} onTest={vi.fn()}
      />
    );
    expect(screen.getByText("ИИ Провайдер")).toBeInTheDocument();
    expect(screen.getByDisplayValue("gemma3:4b")).toBeInTheDocument();
  });

  it("calls onTest on button click", async () => {
    const onTest = vi.fn();
    render(
      <AiProviderSection
        aiProvider="ollama" setAiProvider={vi.fn()}
        aiModel="gemma3:4b" setAiModel={vi.fn()}
        aiApiKey="" setAiApiKey={vi.fn()}
        maskedKey="" showApiKey={false} setShowApiKey={vi.fn()}
        active={false} testing={false} testResult={null}
        setTestResult={vi.fn()} hasKey={false} providerDefaults={defaults} onTest={onTest}
      />
    );
    await userEvent.click(screen.getByText("Проверить подключение"));
    expect(onTest).toHaveBeenCalledTimes(1);
  });

  it("disables test button for non-ollama without key", () => {
    render(
      <AiProviderSection
        aiProvider="claude" setAiProvider={vi.fn()}
        aiModel="claude-sonnet" setAiModel={vi.fn()}
        aiApiKey="" setAiApiKey={vi.fn()}
        maskedKey="" showApiKey={false} setShowApiKey={vi.fn()}
        active={false} testing={false} testResult={null}
        setTestResult={vi.fn()} hasKey={false} providerDefaults={defaults} onTest={vi.fn()}
      />
    );
    expect(screen.getByText("Проверить подключение")).toBeDisabled();
  });

  it("enables test button for ollama without key", () => {
    render(
      <AiProviderSection
        aiProvider="ollama" setAiProvider={vi.fn()}
        aiModel="gemma3:4b" setAiModel={vi.fn()}
        aiApiKey="" setAiApiKey={vi.fn()}
        maskedKey="" showApiKey={false} setShowApiKey={vi.fn()}
        active={false} testing={false} testResult={null}
        setTestResult={vi.fn()} hasKey={false} providerDefaults={defaults} onTest={vi.fn()}
      />
    );
    expect(screen.getByText("Проверить подключение")).not.toBeDisabled();
  });

  it("toggles API key visibility", async () => {
    const setShowApiKey = vi.fn();
    render(
      <AiProviderSection
        aiProvider="ollama" setAiProvider={vi.fn()}
        aiModel="gemma3:4b" setAiModel={vi.fn()}
        aiApiKey="sk-123" setAiApiKey={vi.fn()}
        maskedKey="" showApiKey={false} setShowApiKey={setShowApiKey}
        active={false} testing={false} testResult={null}
        setTestResult={vi.fn()} hasKey={false} providerDefaults={defaults} onTest={vi.fn()}
      />
    );
    const eyeButton = screen.getAllByRole("button")[0];
    await userEvent.click(eyeButton);
    expect(setShowApiKey).toHaveBeenCalledWith(true);
  });
});

// ─── TelegramAccountSection ──────────────────────────────────

describe("TelegramAccountSection", () => {
  const baseProps = {
    phone: "", setPhone: vi.fn(),
    code: "", setCode: vi.fn(),
    loading: false, error: "",
    setError: vi.fn(), onSendCode: vi.fn(),
    onVerify: vi.fn(), onDisconnect: vi.fn(), onReset: vi.fn(),
  };

  it("renders idle state with phone input", () => {
    render(<TelegramAccountSection step="idle" connectedPhone="" {...baseProps} />);
    expect(screen.getByText("TG аккаунт (рассылка)")).toBeInTheDocument();
    expect(screen.getByText("Отправить код")).toBeInTheDocument();
  });

  it("renders connected state with phone", () => {
    render(<TelegramAccountSection step="connected" connectedPhone="+79001234567" {...baseProps} />);
    expect(screen.getByText("+79001234567")).toBeInTheDocument();
    expect(screen.getByText("Отключить")).toBeInTheDocument();
  });

  it("calls onDisconnect", async () => {
    const onDisconnect = vi.fn();
    render(<TelegramAccountSection step="connected" connectedPhone="+79001234567" {...baseProps} onDisconnect={onDisconnect} />);
    await userEvent.click(screen.getByText("Отключить"));
    expect(onDisconnect).toHaveBeenCalledTimes(1);
  });

  it("renders code_sent state with verify", () => {
    render(<TelegramAccountSection step="code_sent" connectedPhone="" {...baseProps} phone="+79001234567" />);
    expect(screen.getByText(/Код отправлен/)).toBeInTheDocument();
    expect(screen.getByText("Подтвердить")).toBeInTheDocument();
  });

  it("calls onVerify with code", async () => {
    const onVerify = vi.fn();
    render(<TelegramAccountSection step="code_sent" connectedPhone="" {...baseProps} code="12345" onVerify={onVerify} />);
    await userEvent.click(screen.getByText("Подтвердить"));
    expect(onVerify).toHaveBeenCalledTimes(1);
  });

  it("calls onReset to go back", async () => {
    const onReset = vi.fn();
    render(<TelegramAccountSection step="code_sent" connectedPhone="" {...baseProps} onReset={onReset} />);
    await userEvent.click(screen.getByText("Ввести другой номер"));
    expect(onReset).toHaveBeenCalledTimes(1);
  });

  it("shows error message", () => {
    render(<TelegramAccountSection step="code_sent" connectedPhone="" {...baseProps} error="Неверный код" />);
    expect(screen.getByText("Неверный код")).toBeInTheDocument();
  });

  it("disables send button when phone too short", () => {
    render(<TelegramAccountSection step="idle" connectedPhone="" {...baseProps} phone="+7900" />);
    expect(screen.getByText("Отправить код")).toBeDisabled();
  });

  it("enables send button with valid phone", () => {
    render(<TelegramAccountSection step="idle" connectedPhone="" {...baseProps} phone="+79001234567" />);
    expect(screen.getByText("Отправить код")).not.toBeDisabled();
  });
});
