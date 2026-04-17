"use client";

import { useState } from "react";
import { Save, Check, X, Loader2, Zap } from "lucide-react";
import type { UserSettings } from "@/lib/api";
import { useSettingsCore, useTelegramBot, useTelegramAccount, useImapSettings, useResendSettings, useSmtpSettings, useAiSettings, PROVIDER_DEFAULTS } from "@/hooks/useSettings";
import { ProfileCard } from "@/components/settings/ProfileCard";
import { NotificationsCard } from "@/components/settings/NotificationsCard";
import { TelegramBotSection } from "@/components/settings/TelegramBotSection";
import { TelegramAccountSection } from "@/components/settings/TelegramAccountSection";
import { ImapSection } from "@/components/settings/ImapSection";
import { ResendSection } from "@/components/settings/ResendSection";
import { SmtpSection } from "@/components/settings/SmtpSection";
import { AiProviderSection } from "@/components/settings/AiProviderSection";

export default function SettingsPage() {
  const core = useSettingsCore();
  const tgBot = useTelegramBot(core.settings, core.setSettings);
  const tgAcc = useTelegramAccount();
  const imap = useImapSettings(core.settings);
  const resend = useResendSettings(core.settings, core.setSettings);
  const smtp = useSmtpSettings(core.settings, core.setSettings);
  const ai = useAiSettings(core.settings, core.setSettings);

  const [notifyTg, setNotifyTg] = useState(true);
  const [notifyEmail, setNotifyEmail] = useState(false);

  if (core.loading) {
    return <div className="flex h-full items-center justify-center"><div className="size-8 animate-spin rounded-full border-2 border-[#004ac6] border-t-transparent" /></div>;
  }

  const handleSave = () => {
    const update: Partial<UserSettings> = {
      imap_host: imap.host, imap_port: imap.port, imap_user: imap.user,
      ai_provider: ai.provider, ai_model: ai.model,
      notify_telegram: notifyTg, notify_email_digest: notifyEmail,
    };
    if (tgBot.tgToken && !tgBot.tgToken.startsWith("...")) update.telegram_bot_token = tgBot.tgToken;
    if (imap.password && !imap.password.startsWith("...")) update.imap_password = imap.password;
    if (resend.key && !resend.key.startsWith("...")) update.resend_api_key = resend.key;
    if (smtp.host) update.smtp_host = smtp.host;
    if (smtp.port) update.smtp_port = smtp.port;
    if (smtp.user) update.smtp_user = smtp.user;
    if (smtp.password && !smtp.password.startsWith("...")) update.smtp_password = smtp.password;
    if (ai.apiKey && !ai.apiKey.startsWith("...")) update.ai_api_key = ai.apiKey;
    core.save(update);
  };

  return (
    <div className="min-h-full px-4 sm:px-8 lg:px-12 pb-16 pt-8 sm:pt-16 lg:pt-24">
      <div className="mx-auto max-w-5xl">
        <header className="mb-10">
          <h2 className="text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">Настройки</h2>
          <p className="mt-2 text-[#434655]">Управление профилем, каналами связи и конфигурацией ИИ</p>
        </header>

        {core.saveResult && (
          <div className={`fixed right-8 top-8 z-50 flex items-center gap-3 rounded-xl px-6 py-4 text-sm font-bold shadow-lg transition-all ${
            core.saveResult === "success" ? "bg-green-600 text-white" : "bg-red-600 text-white"
          }`}>
            {core.saveResult === "success" ? <><Check className="size-5" /> Настройки сохранены</> : <><X className="size-5" /> Ошибка сохранения</>}
          </div>
        )}

        <div className="grid grid-cols-12 gap-8">
          <section className="col-span-12 flex flex-col gap-8 lg:col-span-4">
            <ProfileCard settings={core.settings} />
            <NotificationsCard notifyTg={notifyTg} setNotifyTg={setNotifyTg} notifyEmail={notifyEmail} setNotifyEmail={setNotifyEmail} telegramBotActive={tgBot.botActive} />
            <button onClick={handleSave} disabled={core.saving}
              className={`flex w-full items-center justify-center gap-3 rounded-xl px-6 py-4 text-base font-extrabold text-white shadow-lg transition-all hover:-translate-y-0.5 active:scale-95 disabled:opacity-50 ${
                core.saveResult === "success" ? "bg-green-600 shadow-green-600/30"
                  : core.saveResult === "error" ? "bg-red-600 shadow-red-600/30"
                    : "bg-gradient-to-br from-[#004ac6] to-[#2563eb] shadow-[#004ac6]/30"
              }`}>
              {core.saving ? <Loader2 className="size-5 animate-spin" />
                : core.saveResult === "success" ? <Check className="size-5" />
                  : core.saveResult === "error" ? <X className="size-5" />
                    : <Save className="size-5" />}
              {core.saving ? "Сохраняем..." : core.saveResult === "success" ? "Сохранено!" : core.saveResult === "error" ? "Ошибка!" : "Сохранить изменения"}
            </button>
          </section>

          <div className="col-span-12 flex flex-col gap-8 lg:col-span-8">
            <section className="rounded-xl bg-white p-8 shadow-sm ring-1 ring-[#c3c6d7]/10">
              <div className="mb-8 flex items-center gap-3"><Zap className="size-5 text-[#004ac6]" /><h3 className="text-xl font-bold text-[#0d1c2e]">Каналы связи</h3></div>
              <div className="space-y-10">
                <TelegramBotSection botActive={tgBot.botActive} maskedToken={tgBot.maskedToken} tgToken={tgBot.tgToken} setTgToken={tgBot.setTgToken} saving={tgBot.saving} onConnect={tgBot.connect} />
                <hr className="border-[#c3c6d7]/10" />
                <TelegramAccountSection step={tgAcc.step} connectedPhone={tgAcc.connectedPhone} phone={tgAcc.phone} setPhone={tgAcc.setPhone} code={tgAcc.code} setCode={tgAcc.setCode} loading={tgAcc.loading} error={tgAcc.error} setError={tgAcc.setError} onSendCode={tgAcc.sendCode} onVerify={tgAcc.verify} onDisconnect={tgAcc.disconnect} onReset={tgAcc.reset} />
                <hr className="border-[#c3c6d7]/10" />
                <ImapSection imapHost={imap.host} setImapHost={imap.setHost} imapPort={imap.port} setImapPort={imap.setPort} imapUser={imap.user} setImapUser={imap.setUser} imapPassword={imap.password} setImapPassword={imap.setPassword} maskedPassword={imap.maskedPassword} active={imap.active} testing={imap.testing} testResult={imap.testResult} setTestResult={imap.setTestResult} onTest={imap.test} />
                <hr className="border-[#c3c6d7]/10" />
                <ResendSection maskedKey={resend.maskedKey} resendKey={resend.key} setResendKey={resend.setKey} active={resend.active} testing={resend.testing} testResult={resend.testResult} setTestResult={resend.setTestResult} hasKey={resend.hasKey} onTest={resend.test} />
                <hr className="border-[#c3c6d7]/10" />
                <SmtpSection smtpHost={smtp.host} setSmtpHost={smtp.setHost} smtpPort={smtp.port} setSmtpPort={smtp.setPort} smtpUser={smtp.user} setSmtpUser={smtp.setUser} smtpPassword={smtp.password} setSmtpPassword={smtp.setPassword} maskedPassword={smtp.maskedPassword} active={smtp.active} testing={smtp.testing} testResult={smtp.testResult} setTestResult={smtp.setTestResult} onTest={smtp.test} />
              </div>
            </section>
            <AiProviderSection aiProvider={ai.provider} setAiProvider={ai.setProvider} aiModel={ai.model} setAiModel={ai.setModel} aiApiKey={ai.apiKey} setAiApiKey={ai.setApiKey} maskedKey={ai.maskedKey} showApiKey={ai.showKey} setShowApiKey={ai.setShowKey} active={ai.active} testing={ai.testing} testResult={ai.testResult} setTestResult={ai.setTestResult} hasKey={ai.hasKey} providerDefaults={PROVIDER_DEFAULTS} onTest={ai.test} />
          </div>
        </div>
      </div>
    </div>
  );
}
