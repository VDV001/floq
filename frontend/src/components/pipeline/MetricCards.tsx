import { TrendingUp, Clock, DollarSign, Sparkles, Brain } from "lucide-react";

export function MetricCards({ totalActive, columnCounts }: { totalActive: number; columnCounts: Record<string, number> }) {
  const newCount = columnCounts["new"] || 0;
  const qualifiedCount = columnCounts["qualified"] || 0;
  const conversionPct = newCount > 0 ? Math.round((qualifiedCount / newCount) * 100) : 0;
  const followupCount = columnCounts["followup"] || 0;

  return (
    <div className="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <div className="rounded-xl border border-[#c3c6d7]/5 bg-white p-4 shadow-sm">
        <div className="mb-2 flex items-center gap-2">
          <div className="flex size-8 items-center justify-center rounded-lg bg-blue-500/10"><TrendingUp className="size-4 text-blue-500" /></div>
          <span className="text-xs font-medium text-[#737686]">Конверсия (New &rarr; Qual)</span>
        </div>
        <p className="text-3xl font-bold text-[#0d1c2e]">{conversionPct}%</p>
        <div className="mt-2 h-1.5 w-full rounded-full bg-blue-500/10">
          <div className="h-1.5 rounded-full bg-blue-500" style={{ width: `${Math.min(conversionPct, 100)}%` }} />
        </div>
      </div>
      <div className="rounded-xl border border-[#c3c6d7]/5 bg-white p-4 shadow-sm">
        <div className="mb-2 flex items-center gap-2">
          <div className="flex size-8 items-center justify-center rounded-lg bg-purple-500/10"><Clock className="size-4 text-purple-500" /></div>
          <span className="text-xs font-medium text-[#737686]">Всего активных</span>
        </div>
        <p className="text-3xl font-bold text-[#0d1c2e]">{totalActive}</p>
      </div>
      <div className="rounded-xl border border-[#c3c6d7]/5 bg-white p-4 shadow-sm">
        <div className="mb-2 flex items-center gap-2">
          <div className="flex size-8 items-center justify-center rounded-lg bg-blue-500/10"><DollarSign className="size-4 text-blue-500" /></div>
          <span className="text-xs font-medium text-[#737686]">Новых лидов</span>
        </div>
        <p className="text-3xl font-bold text-[#0d1c2e]">{newCount}</p>
      </div>
      <div className="relative overflow-hidden rounded-xl border border-[#d8d7ff] bg-[#e1e0ff] p-4 shadow-sm">
        <div className="mb-2 flex items-center gap-2">
          <div className="flex size-8 items-center justify-center rounded-lg bg-[#3e3fcc]/10"><Sparkles className="size-4 text-[#3e3fcc]" /></div>
          <span className="text-xs font-semibold text-[#3e3fcc]">Floq AI Инсайт</span>
        </div>
        <p className="text-sm leading-relaxed text-[#2f2ebe]">
          {followupCount > 0 ? `${followupCount} сделок в «Фоллоуап» требуют срочного внимания` : "Все лиды в работе, фоллоуапов нет"}
        </p>
        <Brain className="absolute -bottom-2 -right-2 size-20 text-[#3e3fcc] opacity-10" />
      </div>
    </div>
  );
}
