import { Mail, Send, Sparkles } from "lucide-react";
import { cn } from "@/lib/utils";
import { PIPELINE_STAGES_CONFIG } from "./constants";
import type { InboxLead } from "./constants";

interface PipelineSidebarProps {
  activeStage: string;
  setActiveStage: (s: string) => void;
  statusCounts: Record<string, number>;
  leads: InboxLead[];
  sourceFilter: string;
  setSourceFilter: (v: string) => void;
}

export function PipelineSidebar({ activeStage, setActiveStage, statusCounts, leads, sourceFilter, setSourceFilter }: PipelineSidebarProps) {
  const sources = [...new Set(leads.map((l) => l.sourceName).filter(Boolean))].sort();

  return (
    <nav className="w-72 shrink-0 overflow-y-auto border-r border-[#c3c6d7]/10 bg-[#eff4ff]/50 px-6 py-8 space-y-10">
      <section>
        <h3 className="mb-4 px-2 text-[0.7rem] font-bold uppercase tracking-widest text-[#737686]">Этапы воронки</h3>
        <div className="space-y-1">
          {PIPELINE_STAGES_CONFIG.map((stage) => {
            const Icon = stage.icon;
            const isActive = activeStage === stage.id;
            const count = statusCounts[stage.apiStatus] || 0;
            return (
              <button key={stage.id} onClick={() => setActiveStage(stage.id)}
                className={cn("flex w-full items-center justify-between rounded-xl px-3 py-2.5 text-sm transition-all",
                  isActive ? "bg-white font-bold text-[#004ac6] shadow-sm" : "text-[#434655] hover:bg-[#dce9ff] group")}>
                <div className="flex items-center gap-3"><Icon className="size-5" /><span>{stage.label}</span></div>
                <span className={cn("rounded-full px-2 py-0.5 text-[10px] font-semibold",
                  isActive ? "bg-[#dbe1ff] text-[#004ac6]" : stage.alert && count > 0 ? "bg-[#ffdad6] text-[#93000a]" : "text-[#737686] group-hover:text-[#004ac6]")}>
                  {count}
                </span>
              </button>
            );
          })}
        </div>
      </section>

      <section>
        <h3 className="mb-4 px-2 text-[0.7rem] font-bold uppercase tracking-widest text-[#737686]">Каналы</h3>
        <div className="grid grid-cols-2 gap-2">
          <button className="flex items-center gap-2 rounded-lg border border-[#c3c6d7]/10 bg-white px-3 py-2 text-xs font-medium"><Send className="size-4 text-[#229ED9]" />Telegram</button>
          <button className="flex items-center gap-2 rounded-lg border border-[#c3c6d7]/10 bg-white px-3 py-2 text-xs font-medium"><Mail className="size-4 text-[#004ac6]" />Email</button>
        </div>
      </section>

      {sources.length > 0 && (
        <section>
          <h3 className="mb-4 px-2 text-[0.7rem] font-bold uppercase tracking-widest text-[#737686]">Источник</h3>
          <select value={sourceFilter} onChange={(e) => setSourceFilter(e.target.value)}
            className="w-full rounded-lg border border-[#c3c6d7]/10 bg-white px-3 py-2 text-xs font-medium outline-none focus:ring-2 focus:ring-[#004ac6]/20">
            <option value="">Все источники</option>
            {sources.map((s) => <option key={s} value={s}>{s}</option>)}
          </select>
        </section>
      )}

      <div className="relative overflow-hidden rounded-2xl border border-[#c0c1ff]/30 bg-[#e1e0ff]/40 p-5">
        <div className="absolute -right-4 -top-4 size-16 rounded-full bg-[#585be6]/10 blur-2xl" />
        <div className="mb-3 flex items-center gap-2"><Sparkles className="size-4 text-[#3e3fcc]" /><span className="text-xs font-bold text-[#2f2ebe]">ИИ-сводка</span></div>
        <p className="text-xs italic leading-relaxed text-[#2f2ebe]/80">
          {leads.length === 0
            ? "Нет активных лидов. Подключите Telegram бот в настройках чтобы начать получать обращения."
            : `${leads.length} ${leads.length === 1 ? "лид" : "лидов"} в системе. ${
                (statusCounts["new"] || 0) > 0 ? `${statusCounts["new"]} новых ожидают ответа.`
                  : (statusCounts["followup"] || 0) > 0 ? `${statusCounts["followup"]} требуют фоллоуапа.`
                    : "Все лиды в работе."
              }`}
        </p>
      </div>
    </nav>
  );
}
