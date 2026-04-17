import { Mail, Shield, Loader2 } from "lucide-react";
import { ConnectionBadge } from "./ConnectionBadge";
import { HintIcon } from "./HintIcon";
import { StatusBanner } from "./StatusBanner";

type TestResult = { success: boolean; message?: string; error?: string } | null;

interface ImapSectionProps {
  imapHost: string; setImapHost: (v: string) => void;
  imapPort: string; setImapPort: (v: string) => void;
  imapUser: string; setImapUser: (v: string) => void;
  imapPassword: string; setImapPassword: (v: string) => void;
  maskedPassword: string;
  active: boolean;
  testing: boolean;
  testResult: TestResult;
  setTestResult: (v: TestResult) => void;
  onTest: () => void;
}

export function ImapSection({
  imapHost, setImapHost, imapPort, setImapPort, imapUser, setImapUser,
  imapPassword, setImapPassword, maskedPassword, active, testing, testResult, setTestResult, onTest,
}: ImapSectionProps) {
  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Mail className="size-5 text-[#545f73]" />
          <h4 className="font-bold text-[#0d1c2e]">Email IMAP</h4>
          <HintIcon text="Приём входящих писем. Floq каждую минуту проверяет почту через IMAP. Если ответил проспект из базы — автоматически создаётся лид. Для Gmail нужен пароль приложения." />
        </div>
        <ConnectionBadge active={active} />
      </div>
      <div className="grid grid-cols-12 gap-4">
        <div className="col-span-8">
          <label className="mb-2 block text-xs font-bold uppercase tracking-wide text-[#434655]">Хост</label>
          <input type="text" placeholder="imap.gmail.com" value={imapHost} onChange={(e) => setImapHost(e.target.value)}
            className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20" />
        </div>
        <div className="col-span-4">
          <label className="mb-2 block text-xs font-bold uppercase tracking-wide text-[#434655]">Порт</label>
          <input type="text" placeholder="993" value={imapPort} onChange={(e) => setImapPort(e.target.value)}
            className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20" />
        </div>
        <div className="col-span-6">
          <label className="mb-2 block text-xs font-bold uppercase tracking-wide text-[#434655]">Пользователь</label>
          <input type="text" placeholder="user@example.com" value={imapUser} onChange={(e) => setImapUser(e.target.value)}
            className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20" />
        </div>
        <div className="col-span-6">
          <label className="mb-2 block text-xs font-bold uppercase tracking-wide text-[#434655]">Пароль</label>
          <input type="password" placeholder={maskedPassword || "••••••••••••"} value={imapPassword} onChange={(e) => setImapPassword(e.target.value)}
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
