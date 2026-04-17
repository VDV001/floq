import { Sparkles, Eye, ChevronDown, Info, Loader2, Wifi } from "lucide-react";
import { ConnectionBadge } from "./ConnectionBadge";
import { HintIcon } from "./HintIcon";
import { StatusBanner } from "./StatusBanner";

type TestResult = { success: boolean; message?: string; error?: string } | null;

interface AiProviderSectionProps {
  aiProvider: string; setAiProvider: (v: string) => void;
  aiModel: string; setAiModel: (v: string) => void;
  aiApiKey: string; setAiApiKey: (v: string) => void;
  maskedKey: string;
  showApiKey: boolean; setShowApiKey: (v: boolean) => void;
  active: boolean;
  testing: boolean;
  testResult: TestResult;
  setTestResult: (v: TestResult) => void;
  hasKey: boolean;
  providerDefaults: Record<string, string>;
  onTest: () => void;
}

export function AiProviderSection({
  aiProvider, setAiProvider, aiModel, setAiModel, aiApiKey, setAiApiKey,
  maskedKey, showApiKey, setShowApiKey, active, testing, testResult, setTestResult,
  hasKey, providerDefaults, onTest,
}: AiProviderSectionProps) {
  return (
    <section className="relative rounded-xl bg-white p-8 shadow-sm ring-1 ring-[#c3c6d7]/10">
      <div className="absolute -mr-16 -mt-16 right-0 top-0 size-32 rounded-full bg-[#3e3fcc]/5 blur-3xl" />

      <div className="mb-8 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Sparkles className="size-5 text-[#3e3fcc]" />
          <h3 className="text-xl font-bold text-[#0d1c2e]">ИИ Провайдер</h3>
          <HintIcon text="AI-модель для генерации текстов: квалификация лидов, черновики ответов, холодные письма. Ollama — бесплатно и локально. Claude/OpenAI/Groq — через облако с API-ключом." />
        </div>
        <ConnectionBadge active={active} />
      </div>

      <div className="mb-8 grid grid-cols-2 gap-6">
        <div>
          <label className="mb-2 block text-xs font-bold uppercase tracking-wide text-[#434655]">Провайдер</label>
          <div className="relative">
            <select value={aiProvider} onChange={(e) => {
              const v = e.target.value;
              setAiProvider(v);
              if (providerDefaults[v]) setAiModel(providerDefaults[v]);
            }}
              className="w-full appearance-none rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm outline-none transition-all focus:ring-2 focus:ring-[#3e3fcc]/20">
              <option value="ollama">Ollama (локальная)</option>
              <option value="claude">Claude (Anthropic)</option>
              <option value="openai">OpenAI (GPT)</option>
              <option value="groq">Groq (быстрая)</option>
            </select>
            <ChevronDown className="pointer-events-none absolute right-3 top-3.5 size-4 text-[#434655]" />
          </div>
        </div>
        <div>
          <label className="mb-2 block text-xs font-bold uppercase tracking-wide text-[#434655]">Название модели</label>
          <input type="text" placeholder="gemma3:4b" value={aiModel} onChange={(e) => setAiModel(e.target.value)}
            className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm outline-none transition-all focus:ring-2 focus:ring-[#3e3fcc]/20" />
        </div>
      </div>

      <div className="rounded-xl border border-[#3e3fcc]/10 bg-[#e1e0ff]/30 p-6 backdrop-blur-sm">
        <label className="mb-2 block text-xs font-bold uppercase tracking-wide text-[#3e3fcc]">API Ключ</label>
        <div className="flex gap-3">
          <input type={showApiKey ? "text" : "password"} placeholder={maskedKey || "Не задан"} value={aiApiKey}
            onChange={(e) => setAiApiKey(e.target.value)}
            className="flex-1 rounded-lg border-none bg-white/50 px-4 py-3 font-mono text-sm outline-none transition-all focus:ring-2 focus:ring-[#3e3fcc]/20" />
          <button onClick={() => setShowApiKey(!showApiKey)}
            className="flex size-12 items-center justify-center rounded-lg bg-white/50 text-[#3e3fcc] transition-colors hover:bg-white">
            <Eye className="size-5" />
          </button>
        </div>
        <p className="mt-3 flex items-center gap-1 text-[11px] text-[#2f2ebe]">
          <Info className="size-3.5" />
          Ключ хранится в зашифрованном виде и никогда не передается третьим лицам.
        </p>
      </div>

      <button onClick={onTest} disabled={testing || (aiProvider !== "ollama" && !aiApiKey && !hasKey)}
        className="mt-6 flex w-full items-center justify-center gap-2 rounded-lg bg-[#e1e0ff]/50 py-3 text-sm font-bold text-[#3e3fcc] transition-colors hover:bg-[#e1e0ff] disabled:opacity-50">
        {testing ? <Loader2 className="size-[18px] animate-spin" /> : <Wifi className="size-[18px]" />}
        {testing ? "Проверяем подключение..." : "Проверить подключение"}
      </button>
      <StatusBanner result={testResult} onDismiss={() => setTestResult(null)} />
    </section>
  );
}
