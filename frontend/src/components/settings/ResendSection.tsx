import { Send, Loader2 } from "lucide-react";
import { ConnectionBadge } from "./ConnectionBadge";
import { HintIcon } from "./HintIcon";
import { StatusBanner } from "./StatusBanner";

type TestResult = { success: boolean; message?: string; error?: string } | null;

interface ResendSectionProps {
  maskedKey: string;
  resendKey: string;
  setResendKey: (v: string) => void;
  active: boolean;
  testing: boolean;
  testResult: TestResult;
  setTestResult: (v: TestResult) => void;
  hasKey: boolean;
  onTest: () => void;
}

export function ResendSection({
  maskedKey, resendKey, setResendKey, active, testing, testResult, setTestResult, hasKey, onTest,
}: ResendSectionProps) {
  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Send className="size-5 text-[#0d1c2e]" />
          <h4 className="font-bold text-[#0d1c2e]">Resend API</h4>
          <HintIcon text="Сервис для отправки email через API. Требует верифицированный домен. Бесплатно до 100 писем/день. Альтернатива — SMTP ниже." />
        </div>
        <ConnectionBadge active={active} />
      </div>
      <div className="flex gap-4">
        <input type="password" placeholder={maskedKey || "re_123456789..."} value={resendKey}
          onChange={(e) => setResendKey(e.target.value)}
          className="flex-1 rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm placeholder-[#434655]/50 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20" />
        <button onClick={onTest} disabled={testing || (!resendKey && !hasKey)}
          className="rounded-lg bg-[#2563eb] px-6 py-3 text-sm font-bold text-white transition-all hover:brightness-110 disabled:opacity-50">
          {testing ? <Loader2 className="size-[18px] animate-spin" /> : "Проверить"}
        </button>
      </div>
      <StatusBanner result={testResult} onDismiss={() => setTestResult(null)} />
    </div>
  );
}
