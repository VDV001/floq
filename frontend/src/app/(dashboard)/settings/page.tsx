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
} from "lucide-react";
import { Switch } from "@/components/ui/switch";
import { api, type UserSettings } from "@/lib/api";

export default function SettingsPage() {
  const [settings, setSettings] = useState<UserSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [showApiKey, setShowApiKey] = useState(false);

  // Editable fields (separate from loaded settings to track changes)
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

  useEffect(() => {
    api
      .getSettings()
      .then((data) => {
        setSettings(data);
        // Only set non-masked values for display
        setImapHost(data.imap_host);
        setImapPort(data.imap_port);
        setImapUser(data.imap_user);
        setAiProvider(data.ai_provider);
        setAiModel(data.ai_model);
        setNotifyTg(data.notify_telegram);
        setNotifyEmail(data.notify_email_digest);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const handleSave = async () => {
    setSaving(true);
    setSaved(false);
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
      // Only send sensitive fields if user typed something new (not the masked value)
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
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
    } catch {
      alert("Ошибка сохранения настроек");
    } finally {
      setSaving(false);
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
    <div className="min-h-full px-12 pb-16 pt-24">
      <div className="mx-auto max-w-5xl">
        {/* Header */}
        <header className="mb-10">
          <h2 className="text-4xl font-extrabold tracking-tight text-[#0d1c2e]">
            Настройки
          </h2>
          <p className="mt-2 text-[#434655]">
            Управление профилем, каналами связи и конфигурацией ИИ
          </p>
        </header>

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
                  <span className="text-sm font-medium text-[#434655] transition-colors group-hover:text-[#0d1c2e]">
                    В Telegram о новых лидах
                  </span>
                  <Switch checked={notifyTg} onCheckedChange={setNotifyTg} />
                </label>
                <label className="group flex cursor-pointer items-center justify-between">
                  <span className="text-sm font-medium text-[#434655] transition-colors group-hover:text-[#0d1c2e]">
                    Еженедельный отчет по email
                  </span>
                  <Switch checked={notifyEmail} onCheckedChange={setNotifyEmail} />
                </label>
              </div>
            </div>
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
                    {settings?.telegram_bot_active ? (
                      <span className="flex items-center gap-1 rounded-full bg-green-100 px-3 py-1 text-[10px] font-bold uppercase tracking-wider text-green-700">
                        <Check className="size-3" />
                        Подключен
                      </span>
                    ) : (
                      <span className="rounded-full bg-[#ba1a1a]/10 px-3 py-1 text-[10px] font-bold uppercase tracking-wider text-[#ba1a1a]">
                        Не подключен
                      </span>
                    )}
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
                  <div className="mb-6 flex items-center gap-2">
                    <Mail className="size-5 text-[#545f73]" />
                    <h4 className="font-bold text-[#0d1c2e]">Email IMAP</h4>
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
                  <button className="mt-6 flex w-full items-center justify-center gap-2 rounded-lg bg-[#eff4ff] py-3 text-sm font-bold text-[#0d1c2e] transition-colors hover:bg-[#dce9ff]">
                    <Shield className="size-[18px]" />
                    Тест соединения
                  </button>
                </div>

                <hr className="border-[#c3c6d7]/10" />

                {/* Resend API */}
                <div>
                  <div className="mb-4 flex items-center gap-2">
                    <Send className="size-5 text-[#0d1c2e]" />
                    <h4 className="font-bold text-[#0d1c2e]">Resend API</h4>
                  </div>
                  <input
                    type="password"
                    placeholder={settings?.resend_api_key || "re_123456789..."}
                    value={resendKey}
                    onChange={(e) => setResendKey(e.target.value)}
                    className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
                  />
                </div>
              </div>
            </section>

            {/* AI Provider */}
            <section className="relative overflow-hidden rounded-xl bg-white p-8 shadow-sm ring-1 ring-[#c3c6d7]/10">
              <div className="absolute -mr-16 -mt-16 right-0 top-0 size-32 rounded-full bg-[#3e3fcc]/5 blur-3xl" />

              <div className="mb-8 flex items-center gap-3">
                <Sparkles className="size-5 text-[#3e3fcc]" />
                <h3 className="text-xl font-bold text-[#0d1c2e]">
                  ИИ Провайдер
                </h3>
              </div>

              <div className="mb-8 grid grid-cols-2 gap-6">
                <div>
                  <label className="mb-2 block text-xs font-bold uppercase tracking-wide text-[#434655]">
                    Модель ИИ
                  </label>
                  <div className="relative">
                    <select
                      value={aiProvider}
                      onChange={(e) => setAiProvider(e.target.value)}
                      className="w-full appearance-none rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm outline-none transition-all focus:ring-2 focus:ring-[#3e3fcc]/20"
                    >
                      <option value="ollama">Ollama (локальная)</option>
                      <option value="claude">Claude (Anthropic)</option>
                      <option value="openai">OpenAI (GPT)</option>
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
            </section>

            {/* Save */}
            <div className="flex justify-end pt-4">
              <button
                onClick={handleSave}
                disabled={saving}
                className="flex items-center gap-3 rounded-xl bg-gradient-to-br from-[#004ac6] to-[#2563eb] px-12 py-4 text-lg font-extrabold text-white shadow-xl shadow-[#004ac6]/30 transition-all hover:-translate-y-0.5 active:scale-95 disabled:opacity-50"
              >
                {saving ? (
                  <Loader2 className="size-5 animate-spin" />
                ) : saved ? (
                  <Check className="size-5" />
                ) : (
                  <Save className="size-5" />
                )}
                {saved ? "Сохранено!" : "Сохранить изменения"}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
