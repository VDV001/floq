"use client";

import { useState, useEffect } from "react";
import {
  Lock,
  Camera,
  Bell,
  Zap,
  Mail,
  Send,
  Sparkles,
  Eye,
  Save,
  Info,
  ChevronDown,
  Shield,
  Check,
  Loader2,
  X,
  Wifi,
} from "lucide-react";
import { Switch } from "@/components/ui/switch";
import { api, type UserSettings } from "@/lib/api";

type TestResult = { success: boolean; message?: string; error?: string } | null;

function ConnectionBadge({ active }: { active: boolean }) {
  return active ? (
    <span className="flex items-center gap-1 rounded-full bg-green-100 px-3 py-1 text-[10px] font-bold uppercase tracking-wider text-green-700">
      <Check className="size-3" />
      Подключен
    </span>
  ) : (
    <span className="rounded-full bg-[#ba1a1a]/10 px-3 py-1 text-[10px] font-bold uppercase tracking-wider text-[#ba1a1a]">
      Не подключен
    </span>
  );
}

function StatusBanner({ result, onDismiss }: { result: TestResult; onDismiss: () => void }) {
  if (!result) return null;
  return (
    <div
      className={`mt-3 flex items-center justify-between rounded-lg px-4 py-2.5 text-sm font-medium ${
        result.success
          ? "bg-green-50 text-green-700 ring-1 ring-green-200"
          : "bg-red-50 text-red-600 ring-1 ring-red-200"
      }`}
    >
      <span className="flex items-center gap-2">
        {result.success ? <Check className="size-4" /> : <X className="size-4" />}
        {result.success ? result.message : result.error}
      </span>
      <button onClick={onDismiss} className="ml-2 opacity-60 hover:opacity-100">
        <X className="size-3.5" />
      </button>
    </div>
  );
}

export default function SettingsPage() {
  const [settings, setSettings] = useState<UserSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saveResult, setSaveResult] = useState<"success" | "error" | null>(null);
  const [showApiKey, setShowApiKey] = useState(false);

  // Editable fields
  const [tgToken, setTgToken] = useState("");
  const [imapHost, setImapHost] = useState("");
  const [imapPort, setImapPort] = useState("993");
  const [imapUser, setImapUser] = useState("");
  const [imapPassword, setImapPassword] = useState("");
  const [resendKey, setResendKey] = useState("");
  const [aiProvider, setAiProvider] = useState("ollama");
  const [aiModel, setAiModel] = useState("gemma3:4b");
  const [aiApiKey, setAiApiKey] = useState("");
  const [notifyTg, setNotifyTg] = useState(true);
  const [notifyEmail, setNotifyEmail] = useState(false);

  // Test states
  const [imapTesting, setImapTesting] = useState(false);
  const [imapTestResult, setImapTestResult] = useState<TestResult>(null);
  const [imapVerified, setImapVerified] = useState<boolean | null>(null);
  const [resendTesting, setResendTesting] = useState(false);
  const [resendTestResult, setResendTestResult] = useState<TestResult>(null);
  const [resendVerified, setResendVerified] = useState<boolean | null>(null);
  const [aiTesting, setAiTesting] = useState(false);
  const [aiTestResult, setAiTestResult] = useState<TestResult>(null);
  const [aiVerified, setAiVerified] = useState<boolean | null>(null);

  const providerDefaults: Record<string, string> = {
    ollama: "gemma3:4b",
    claude: "claude-sonnet-4-20250514",
    openai: "gpt-4o",
    groq: "openai/gpt-oss-120b",
  };

  useEffect(() => {
    api
      .getSettings()
      .then((data) => {
        setSettings(data);
        setImapHost(data.imap_host);
        setImapPort(data.imap_port);
        setImapUser(data.imap_user);
        setAiProvider(data.ai_provider || "ollama");
        setAiModel(data.ai_model || providerDefaults[data.ai_provider] || "gemma3:4b");
        setNotifyTg(data.notify_telegram);
        setNotifyEmail(data.notify_email_digest);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleSave = async () => {
    setSaving(true);
    setSaveResult(null);
    try {
      const update: Partial<UserSettings> = {
        imap_host: imapHost,
        imap_port: imapPort,
        imap_user: imapUser,
        ai_provider: aiProvider,
        ai_model: aiModel,
        notify_telegram: notifyTg,
        notify_email_digest: notifyEmail,
      };
      if (tgToken && !tgToken.startsWith("...")) {
        update.telegram_bot_token = tgToken;
      }
      if (imapPassword && !imapPassword.startsWith("...")) {
        update.imap_password = imapPassword;
      }
      if (resendKey && !resendKey.startsWith("...")) {
        update.resend_api_key = resendKey;
      }
      if (aiApiKey && !aiApiKey.startsWith("...")) {
        update.ai_api_key = aiApiKey;
      }

      const updated = await api.updateSettings(update);
      setSettings(updated);
      setSaveResult("success");
      setTimeout(() => setSaveResult(null), 4000);
    } catch {
      setSaveResult("error");
      setTimeout(() => setSaveResult(null), 4000);
    } finally {
      setSaving(false);
    }
  };

  const handleTestIMAP = async () => {
    setImapTesting(true);
    setImapTestResult(null);
    try {
      const host = imapHost || settings?.imap_host || "";
      const port = imapPort || settings?.imap_port || "993";
      const user = imapUser || settings?.imap_user || "";
      const password = imapPassword && !imapPassword.startsWith("...") ? imapPassword : "";
      if (!host || !user) {
        setImapTestResult({ success: false, error: "Заполните хост и пользователя IMAP" });
        return;
      }
      const res = await api.testIMAP(host, port, user, password || "", !password);
      setImapTestResult(res);
      setImapVerified(res.success);
    } catch {
      setImapTestResult({ success: false, error: "Ошибка запроса" });
      setImapVerified(false);
    } finally {
      setImapTesting(false);
    }
  };

  const handleTestResend = async () => {
    setResendTesting(true);
    setResendTestResult(null);
    try {
      const key = resendKey && !resendKey.startsWith("...") ? resendKey : "";
      const res = await api.testResend(key, !key);
      setResendTestResult(res);
      setResendVerified(res.success);
      // Auto-save new key on successful test
      if (res.success && key) {
        const updated = await api.updateSettings({ resend_api_key: key });
        setSettings(updated);
        setResendKey("");
      }
    } catch {
      setResendTestResult({ success: false, error: "Ошибка запроса" });
      setResendVerified(false);
    } finally {
      setResendTesting(false);
    }
  };

  const handleTestAI = async () => {
    setAiTesting(true);
    setAiTestResult(null);
    try {
      const key = aiApiKey && !aiApiKey.startsWith("...") ? aiApiKey : "";
      const res = await api.testAI(aiProvider, aiModel, key, !key);
      setAiTestResult(res);
      setAiVerified(res.success);
      // Auto-save provider/model/key on successful test
      if (res.success) {
        const update: Partial<typeof settings & object> = {
          ai_provider: aiProvider,
          ai_model: aiModel,
        };
        if (key) update.ai_api_key = key;
        const updated = await api.updateSettings(update);
        setSettings(updated);
        if (key) setAiApiKey("");
      }
    } catch {
      setAiTestResult({ success: false, error: "Ошибка запроса" });
      setAiVerified(false);
    } finally {
      setAiTesting(false);
    }
  };

  const handleConnectTelegram = async () => {
    if (!tgToken || tgToken.startsWith("...")) return;
    setSaving(true);
    try {
      const updated = await api.updateSettings({
        telegram_bot_token: tgToken,
      });
      setSettings(updated);
      setTgToken("");
    } catch {
      alert("Не удалось подключить бота. Проверьте токен.");
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="size-8 animate-spin rounded-full border-2 border-[#004ac6] border-t-transparent" />
      </div>
    );
  }

  const initials = settings?.full_name
    ? settings.full_name
        .split(" ")
        .map((w) => w[0])
        .join("")
        .toUpperCase()
        .slice(0, 2)
    : "??";

  return (
    <div className="min-h-full px-4 sm:px-8 lg:px-12 pb-16 pt-8 sm:pt-16 lg:pt-24">
      <div className="mx-auto max-w-5xl">
        {/* Header */}
        <header className="mb-10">
          <h2 className="text-2xl sm:text-3xl lg:text-4xl font-extrabold tracking-tight text-[#0d1c2e]">
            Настройки
          </h2>
          <p className="mt-2 text-[#434655]">
            Управление профилем, каналами связи и конфигурацией ИИ
          </p>
        </header>

        {/* Save result toast */}
        {saveResult && (
          <div
            className={`fixed right-8 top-8 z-50 flex items-center gap-3 rounded-xl px-6 py-4 text-sm font-bold shadow-lg transition-all ${
              saveResult === "success"
                ? "bg-green-600 text-white"
                : "bg-red-600 text-white"
            }`}
          >
            {saveResult === "success" ? (
              <><Check className="size-5" /> Настройки сохранены</>
            ) : (
              <><X className="size-5" /> Ошибка сохранения</>
            )}
          </div>
        )}

        <div className="grid grid-cols-12 gap-8">
          {/* ── Left column: Profile + Notifications ── */}
          <section className="col-span-12 flex flex-col gap-8 lg:col-span-4">
            {/* Profile */}
            <div className="rounded-xl bg-white p-8 shadow-sm ring-1 ring-[#c3c6d7]/10">
              <div className="flex flex-col items-center text-center">
                <div className="group relative mb-4 cursor-pointer">
                  <div className="flex size-24 items-center justify-center rounded-full border-4 border-[#eff4ff] bg-[#dbe1ff] text-2xl font-bold text-[#004ac6] shadow-md">
                    {initials}
                  </div>
                  <div className="absolute inset-0 flex items-center justify-center rounded-full bg-black/40 opacity-0 transition-opacity group-hover:opacity-100">
                    <Camera className="size-5 text-white" />
                  </div>
                </div>
                <h3 className="text-xl font-bold text-[#0d1c2e]">
                  {settings?.full_name || "—"}
                </h3>
                <p className="mb-6 text-sm text-[#434655]">
                  {settings?.email || "—"}
                </p>
                <button className="flex w-full items-center justify-center gap-2 rounded-lg bg-[#eff4ff] px-4 py-2.5 text-sm font-semibold text-[#434655] transition-colors hover:bg-[#dce9ff]">
                  <Lock className="size-[18px]" />
                  Сменить пароль
                </button>
              </div>
            </div>

            {/* Notifications */}
            <div className="rounded-xl bg-white p-8 shadow-sm ring-1 ring-[#c3c6d7]/10">
              <div className="mb-6 flex items-center gap-3">
                <Bell className="size-5 text-[#004ac6]" />
                <h3 className="text-lg font-bold text-[#0d1c2e]">
                  Уведомления
                </h3>
              </div>
              <div className="space-y-6">
                <label className="group flex cursor-pointer items-center justify-between">
                  <div>
                    <span className="text-sm font-medium text-[#434655] transition-colors group-hover:text-[#0d1c2e]">
                      В Telegram о новых лидах
                    </span>
                    <p className="text-xs text-[#434655]/60">
                      {settings?.telegram_bot_active
                        ? "Бот подключен — уведомления будут приходить"
                        : "Сначала подключите Telegram бота"}
                    </p>
                  </div>
                  <Switch
                    checked={notifyTg}
                    onCheckedChange={setNotifyTg}
                    disabled={!settings?.telegram_bot_active}
                  />
                </label>
                <label className="group flex cursor-pointer items-center justify-between">
                  <div>
                    <span className="text-sm font-medium text-[#434655] transition-colors group-hover:text-[#0d1c2e]">
                      Еженедельный отчет по email
                    </span>
                    <p className="text-xs text-[#434655]/60">
                      Каждый понедельник — сводка по лидам и воронке
                    </p>
                  </div>
                  <Switch checked={notifyEmail} onCheckedChange={setNotifyEmail} />
                </label>
              </div>
              <p className="mt-6 text-[11px] text-[#434655]/50">
                Изменения применяются после нажатия «Сохранить»
              </p>
            </div>

            {/* Save button */}
            <button
              onClick={handleSave}
              disabled={saving}
              className={`flex w-full items-center justify-center gap-3 rounded-xl px-6 py-4 text-base font-extrabold text-white shadow-lg transition-all hover:-translate-y-0.5 active:scale-95 disabled:opacity-50 ${
                saveResult === "success"
                  ? "bg-green-600 shadow-green-600/30"
                  : saveResult === "error"
                    ? "bg-red-600 shadow-red-600/30"
                    : "bg-gradient-to-br from-[#004ac6] to-[#2563eb] shadow-[#004ac6]/30"
              }`}
            >
              {saving ? (
                <Loader2 className="size-5 animate-spin" />
              ) : saveResult === "success" ? (
                <Check className="size-5" />
              ) : saveResult === "error" ? (
                <X className="size-5" />
              ) : (
                <Save className="size-5" />
              )}
              {saving
                ? "Сохраняем..."
                : saveResult === "success"
                  ? "Сохранено!"
                  : saveResult === "error"
                    ? "Ошибка!"
                    : "Сохранить изменения"}
            </button>
          </section>

          {/* ── Right column: Channels + AI Provider ── */}
          <div className="col-span-12 flex flex-col gap-8 lg:col-span-8">
            {/* Channels */}
            <section className="rounded-xl bg-white p-8 shadow-sm ring-1 ring-[#c3c6d7]/10">
              <div className="mb-8 flex items-center gap-3">
                <Zap className="size-5 text-[#004ac6]" />
                <h3 className="text-xl font-bold text-[#0d1c2e]">
                  Каналы связи
                </h3>
              </div>

              <div className="space-y-10">
                {/* Telegram */}
                <div>
                  <div className="mb-4 flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Send className="size-5 text-[#229ED9]" />
                      <h4 className="font-bold text-[#0d1c2e]">Telegram bot</h4>
                    </div>
                    <ConnectionBadge active={!!settings?.telegram_bot_active} />
                  </div>
                  <div className="flex gap-4">
                    <input
                      type="password"
                      placeholder={
                        settings?.telegram_bot_token
                          ? settings.telegram_bot_token
                          : "Введите токен..."
                      }
                      value={tgToken}
                      onChange={(e) => setTgToken(e.target.value)}
                      className="flex-1 rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm placeholder-[#434655]/50 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
                    />
                    <button
                      onClick={handleConnectTelegram}
                      disabled={saving || !tgToken || tgToken.startsWith("...")}
                      className="rounded-lg bg-[#2563eb] px-6 py-3 text-sm font-bold text-white transition-all hover:brightness-110 disabled:opacity-50"
                    >
                      {saving ? "..." : "Подключить"}
                    </button>
                  </div>
                </div>

                <hr className="border-[#c3c6d7]/10" />

                {/* Email IMAP */}
                <div>
                  <div className="mb-6 flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Mail className="size-5 text-[#545f73]" />
                      <h4 className="font-bold text-[#0d1c2e]">Email IMAP</h4>
                    </div>
                    <ConnectionBadge active={imapVerified ?? !!settings?.imap_active} />
                  </div>
                  <div className="grid grid-cols-12 gap-4">
                    <div className="col-span-8">
                      <label className="mb-2 block text-xs font-bold uppercase tracking-wide text-[#434655]">
                        Хост
                      </label>
                      <input
                        type="text"
                        placeholder="imap.gmail.com"
                        value={imapHost}
                        onChange={(e) => setImapHost(e.target.value)}
                        className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
                      />
                    </div>
                    <div className="col-span-4">
                      <label className="mb-2 block text-xs font-bold uppercase tracking-wide text-[#434655]">
                        Порт
                      </label>
                      <input
                        type="text"
                        placeholder="993"
                        value={imapPort}
                        onChange={(e) => setImapPort(e.target.value)}
                        className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
                      />
                    </div>
                    <div className="col-span-6">
                      <label className="mb-2 block text-xs font-bold uppercase tracking-wide text-[#434655]">
                        Пользователь
                      </label>
                      <input
                        type="text"
                        placeholder="user@example.com"
                        value={imapUser}
                        onChange={(e) => setImapUser(e.target.value)}
                        className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
                      />
                    </div>
                    <div className="col-span-6">
                      <label className="mb-2 block text-xs font-bold uppercase tracking-wide text-[#434655]">
                        Пароль
                      </label>
                      <input
                        type="password"
                        placeholder={settings?.imap_password || "••••••••••••"}
                        value={imapPassword}
                        onChange={(e) => setImapPassword(e.target.value)}
                        className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
                      />
                    </div>
                  </div>
                  <button
                    onClick={handleTestIMAP}
                    disabled={imapTesting}
                    className="mt-6 flex w-full items-center justify-center gap-2 rounded-lg bg-[#eff4ff] py-3 text-sm font-bold text-[#0d1c2e] transition-colors hover:bg-[#dce9ff] disabled:opacity-50"
                  >
                    {imapTesting ? <Loader2 className="size-[18px] animate-spin" /> : <Shield className="size-[18px]" />}
                    {imapTesting ? "Проверяем..." : "Тест соединения"}
                  </button>
                  <StatusBanner result={imapTestResult} onDismiss={() => setImapTestResult(null)} />
                </div>

                <hr className="border-[#c3c6d7]/10" />

                {/* Resend API */}
                <div>
                  <div className="mb-4 flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Send className="size-5 text-[#0d1c2e]" />
                      <h4 className="font-bold text-[#0d1c2e]">Resend API</h4>
                    </div>
                    <ConnectionBadge active={resendVerified ?? !!settings?.resend_active} />
                  </div>
                  <div className="flex gap-4">
                    <input
                      type="password"
                      placeholder={settings?.resend_api_key || "re_123456789..."}
                      value={resendKey}
                      onChange={(e) => setResendKey(e.target.value)}
                      className="flex-1 rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm placeholder-[#434655]/50 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
                    />
                    <button
                      onClick={handleTestResend}
                      disabled={resendTesting || (!resendKey && !settings?.resend_api_key)}
                      className="rounded-lg bg-[#2563eb] px-6 py-3 text-sm font-bold text-white transition-all hover:brightness-110 disabled:opacity-50"
                    >
                      {resendTesting ? <Loader2 className="size-[18px] animate-spin" /> : "Проверить"}
                    </button>
                  </div>
                  <StatusBanner result={resendTestResult} onDismiss={() => setResendTestResult(null)} />
                </div>
              </div>
            </section>

            {/* AI Provider */}
            <section className="relative overflow-hidden rounded-xl bg-white p-8 shadow-sm ring-1 ring-[#c3c6d7]/10">
              <div className="absolute -mr-16 -mt-16 right-0 top-0 size-32 rounded-full bg-[#3e3fcc]/5 blur-3xl" />

              <div className="mb-8 flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Sparkles className="size-5 text-[#3e3fcc]" />
                  <h3 className="text-xl font-bold text-[#0d1c2e]">
                    ИИ Провайдер
                  </h3>
                </div>
                <ConnectionBadge active={aiVerified ?? !!settings?.ai_active} />
              </div>

              <div className="mb-8 grid grid-cols-2 gap-6">
                <div>
                  <label className="mb-2 block text-xs font-bold uppercase tracking-wide text-[#434655]">
                    Провайдер
                  </label>
                  <div className="relative">
                    <select
                      value={aiProvider}
                      onChange={(e) => {
                        const v = e.target.value;
                        setAiProvider(v);
                        if (providerDefaults[v]) setAiModel(providerDefaults[v]);
                      }}
                      className="w-full appearance-none rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm outline-none transition-all focus:ring-2 focus:ring-[#3e3fcc]/20"
                    >
                      <option value="ollama">Ollama (локальная)</option>
                      <option value="claude">Claude (Anthropic)</option>
                      <option value="openai">OpenAI (GPT)</option>
                      <option value="groq">Groq (быстрая)</option>
                    </select>
                    <ChevronDown className="pointer-events-none absolute right-3 top-3.5 size-4 text-[#434655]" />
                  </div>
                </div>
                <div>
                  <label className="mb-2 block text-xs font-bold uppercase tracking-wide text-[#434655]">
                    Название модели
                  </label>
                  <input
                    type="text"
                    placeholder="gemma3:4b"
                    value={aiModel}
                    onChange={(e) => setAiModel(e.target.value)}
                    className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm outline-none transition-all focus:ring-2 focus:ring-[#3e3fcc]/20"
                  />
                </div>
              </div>

              {/* API Key */}
              <div className="rounded-xl border border-[#3e3fcc]/10 bg-[#e1e0ff]/30 p-6 backdrop-blur-sm">
                <label className="mb-2 block text-xs font-bold uppercase tracking-wide text-[#3e3fcc]">
                  API Ключ
                </label>
                <div className="flex gap-3">
                  <input
                    type={showApiKey ? "text" : "password"}
                    placeholder={settings?.ai_api_key || "Не задан"}
                    value={aiApiKey}
                    onChange={(e) => setAiApiKey(e.target.value)}
                    className="flex-1 rounded-lg border-none bg-white/50 px-4 py-3 font-mono text-sm outline-none transition-all focus:ring-2 focus:ring-[#3e3fcc]/20"
                  />
                  <button
                    onClick={() => setShowApiKey(!showApiKey)}
                    className="flex size-12 items-center justify-center rounded-lg bg-white/50 text-[#3e3fcc] transition-colors hover:bg-white"
                  >
                    <Eye className="size-5" />
                  </button>
                </div>
                <p className="mt-3 flex items-center gap-1 text-[11px] text-[#2f2ebe]">
                  <Info className="size-3.5" />
                  Ключ хранится в зашифрованном виде и никогда не передается третьим лицам.
                </p>
              </div>

              {/* Test AI button */}
              <button
                onClick={handleTestAI}
                disabled={aiTesting || (aiProvider !== "ollama" && !aiApiKey && !settings?.ai_api_key)}
                className="mt-6 flex w-full items-center justify-center gap-2 rounded-lg bg-[#e1e0ff]/50 py-3 text-sm font-bold text-[#3e3fcc] transition-colors hover:bg-[#e1e0ff] disabled:opacity-50"
              >
                {aiTesting ? <Loader2 className="size-[18px] animate-spin" /> : <Wifi className="size-[18px]" />}
                {aiTesting ? "Проверяем подключение..." : "Проверить подключение"}
              </button>
              <StatusBanner result={aiTestResult} onDismiss={() => setAiTestResult(null)} />
            </section>

          </div>
        </div>
      </div>
    </div>
  );
}
