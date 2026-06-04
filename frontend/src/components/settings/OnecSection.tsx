import { Database, Shield, Loader2, RefreshCw, Save, AlertTriangle } from "lucide-react";
import type { OnecAuthType } from "@/lib/api";
import { ConnectionBadge } from "./ConnectionBadge";
import { HintIcon } from "./HintIcon";
import { StatusBanner } from "./StatusBanner";

type TestResult = { success: boolean; error?: string } | null;

interface OnecSectionProps {
  baseURL: string;
  setBaseURL: (v: string) => void;
  authType: OnecAuthType;
  setAuthType: (v: OnecAuthType) => void;
  authSecret: string;
  setAuthSecret: (v: string) => void;
  maskedSecret: string;
  isActive: boolean;
  setIsActive: (v: boolean) => void;
  maskedWebhook: string;
  fullWebhook: string | null;
  regenerating: boolean;
  onRegenerate: () => void;
  saving: boolean;
  saveResult: "success" | "error" | null;
  onSave: () => void;
  testing: boolean;
  testResult: TestResult;
  setTestResult: (v: TestResult) => void;
  onTest: () => void;
}

const inputClass =
  "w-full rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20";
const labelClass = "mb-2 block text-xs font-bold uppercase tracking-wide text-[#434655]";

export function OnecSection(props: OnecSectionProps) {
  const {
    baseURL, setBaseURL, authType, setAuthType, authSecret, setAuthSecret, maskedSecret,
    isActive, setIsActive, maskedWebhook, fullWebhook, regenerating, onRegenerate,
    saving, saveResult, onSave, testing, testResult, setTestResult, onTest,
  } = props;

  const bannerResult = testResult
    ? { success: testResult.success, message: testResult.success ? "Соединение с 1С установлено" : undefined, error: testResult.error }
    : null;

  return (
    <section className="rounded-xl bg-white p-8 shadow-sm ring-1 ring-[#c3c6d7]/10">
      <div className="mb-6 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Database className="size-5 text-[#004ac6]" />
          <h3 className="text-xl font-bold text-[#0d1c2e]">Интеграция с 1С</h3>
          <HintIcon text="Двусторонняя синхронизация с 1С. Адрес OData-сервиса и учётные данные для исходящих вызовов; webhook-секрет аутентифицирует входящие события из 1С. Секреты хранятся отдельно и не возвращаются в открытом виде." />
        </div>
        <ConnectionBadge active={isActive} />
      </div>

      <div className="grid grid-cols-12 gap-4">
        <div className="col-span-8">
          <label className={labelClass} htmlFor="onec-base-url">Адрес OData-сервиса 1С</label>
          <input id="onec-base-url" type="text" placeholder="https://1c.example.com/odata/standard.odata"
            value={baseURL} onChange={(e) => setBaseURL(e.target.value)} className={inputClass} />
        </div>
        <div className="col-span-4">
          <label className={labelClass} htmlFor="onec-auth-type">Авторизация</label>
          <select id="onec-auth-type" value={authType} onChange={(e) => setAuthType(e.target.value as OnecAuthType)} className={inputClass}>
            <option value="basic">Basic</option>
            <option value="token">Bearer-токен</option>
          </select>
        </div>
        <div className="col-span-12">
          <label className={labelClass} htmlFor="onec-secret">
            {authType === "token" ? "Токен" : "Секрет (base64 user:pass)"}
          </label>
          <input id="onec-secret" type="password" aria-label="Секрет авторизации 1С"
            placeholder={maskedSecret || "не задан"} value={authSecret}
            onChange={(e) => setAuthSecret(e.target.value)} className={inputClass} />
        </div>
      </div>

      <label className="mt-5 flex items-center gap-3 text-sm font-medium text-[#0d1c2e]">
        <input type="checkbox" aria-label="Включить интеграцию" checked={isActive}
          onChange={(e) => setIsActive(e.target.checked)} className="size-4 rounded accent-[#004ac6]" />
        Включить приём и отправку событий 1С
      </label>

      <div className="mt-6 rounded-lg bg-[#eff4ff] p-4">
        <div className="flex items-center justify-between gap-3">
          <div>
            <div className={labelClass + " mb-1"}>Webhook-секрет</div>
            <div className="font-mono text-sm text-[#434655]">{maskedWebhook || "не сгенерирован"}</div>
          </div>
          <button onClick={onRegenerate} disabled={regenerating}
            className="flex items-center gap-2 rounded-lg bg-white px-3 py-2 text-sm font-bold text-[#0d1c2e] transition-colors hover:bg-[#dce9ff] disabled:opacity-50">
            {regenerating ? <Loader2 className="size-4 animate-spin" /> : <RefreshCw className="size-4" />}
            Сгенерировать заново
          </button>
        </div>
        {fullWebhook && (
          <div className="mt-3 rounded-lg bg-amber-50 p-3 ring-1 ring-amber-200">
            <div className="flex items-center gap-2 text-xs font-bold text-amber-700">
              <AlertTriangle className="size-4" />
              Скопируйте секрет сейчас — он показывается один раз.
            </div>
            <code className="mt-2 block break-all font-mono text-xs text-amber-900">{fullWebhook}</code>
          </div>
        )}
      </div>

      <div className="mt-6 flex gap-3">
        <button onClick={onTest} disabled={testing}
          className="flex flex-1 items-center justify-center gap-2 rounded-lg bg-[#eff4ff] py-3 text-sm font-bold text-[#0d1c2e] transition-colors hover:bg-[#dce9ff] disabled:opacity-50">
          {testing ? <Loader2 className="size-[18px] animate-spin" /> : <Shield className="size-[18px]" />}
          {testing ? "Проверяем..." : "Тест соединения"}
        </button>
        <button onClick={onSave} disabled={saving}
          className={`flex flex-1 items-center justify-center gap-2 rounded-lg py-3 text-sm font-bold text-white transition-colors disabled:opacity-50 ${
            saveResult === "success" ? "bg-green-600" : saveResult === "error" ? "bg-red-600" : "bg-[#004ac6] hover:bg-[#0039a6]"
          }`}>
          {saving ? <Loader2 className="size-[18px] animate-spin" /> : <Save className="size-[18px]" />}
          {saving ? "Сохраняем..." : "Сохранить"}
        </button>
      </div>
      <StatusBanner result={bannerResult} onDismiss={() => setTestResult(null)} />
    </section>
  );
}
