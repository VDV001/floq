import { Mail, Shield, Loader2 } from "lucide-react";
import { ConnectionBadge } from "./ConnectionBadge";
import { HintIcon } from "./HintIcon";
import { StatusBanner } from "./StatusBanner";

type TestResult = { success: boolean; message?: string; error?: string } | null;

interface SmtpSectionProps {
  smtpHost: string; setSmtpHost: (v: string) => void;
  smtpPort: string; setSmtpPort: (v: string) => void;
  smtpUser: string; setSmtpUser: (v: string) => void;
  smtpPassword: string; setSmtpPassword: (v: string) => void;
  maskedPassword: string;
  active: boolean;
  testing: boolean;
  testResult: TestResult;
  setTestResult: (v: TestResult) => void;
  onTest: () => void;
}

export function SmtpSection({
  smtpHost, setSmtpHost, smtpPort, setSmtpPort, smtpUser, setSmtpUser,
  smtpPassword, setSmtpPassword, maskedPassword, active, testing, testResult, setTestResult, onTest,
}: SmtpSectionProps) {
  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Mail className="size-5 text-[#545f73]" />
          <h4 className="font-bold text-[#0d1c2e]">SMTP (отправка писем)</h4>
          <HintIcon text="Отправка холодных писем через обычную почту (mail.ru, Яндекс, Gmail). Используйте пароль приложения, не обычный пароль аккаунта. Если настроен SMTP — Resend не нужен." />
        </div>
        <ConnectionBadge active={active} />
      </div>
      <p className="mb-4 text-xs text-[#434655]/70">Альтернатива Resend — отправка через mail.ru, Яндекс, Gmail и др.</p>
      <div className="grid grid-cols-12 gap-4">
        <div className="col-span-8">
          <label className="mb-2 block text-xs font-bold uppercase tracking-wide text-[#434655]">Хост</label>
          <input type="text" placeholder="smtp.mail.ru" value={smtpHost} onChange={(e) => setSmtpHost(e.target.value)}
            className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20" />
        </div>
        <div className="col-span-4">
          <label className="mb-2 block text-xs font-bold uppercase tracking-wide text-[#434655]">Порт</label>
          <input type="text" placeholder="465" value={smtpPort} onChange={(e) => setSmtpPort(e.target.value)}
            className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20" />
        </div>
        <div className="col-span-6">
          <label className="mb-2 block text-xs font-bold uppercase tracking-wide text-[#434655]">Email</label>
          <input type="text" placeholder="hello@yourdomain.com" value={smtpUser} onChange={(e) => setSmtpUser(e.target.value)}
            className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20" />
        </div>
        <div className="col-span-6">
          <label className="mb-2 block text-xs font-bold uppercase tracking-wide text-[#434655]">Пароль</label>
          <input type="password" placeholder={maskedPassword || "••••••••••••"} value={smtpPassword} onChange={(e) => setSmtpPassword(e.target.value)}
            className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20" />
        </div>
      </div>
      <button onClick={onTest} disabled={testing}
        className="mt-6 flex w-full items-center justify-center gap-2 rounded-lg bg-[#eff4ff] py-3 text-sm font-bold text-[#0d1c2e] transition-colors hover:bg-[#dce9ff] disabled:opacity-50">
        {testing ? <Loader2 className="size-[18px] animate-spin" /> : <Shield className="size-[18px]" />}
        {testing ? "Проверяем..." : "Тест соединения"}
      </button>
      <StatusBanner result={testResult} onDismiss={() => setTestResult(null)} />
    </div>
  );
}
