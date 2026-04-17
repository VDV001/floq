import { useState } from "react";
import { X, Target, DollarSign, Calendar, Zap } from "lucide-react";
import { api } from "@/lib/api";
import type { PipelineLead } from "./constants";
import { ChannelBadge } from "./ChannelBadge";

export function LeadCard({ lead }: { lead: PipelineLead }) {
  const [expanded, setExpanded] = useState(false);
  const [qual, setQual] = useState<{
    identified_need: string; estimated_budget: string; deadline: string;
    score: number; score_reason: string; recommended_action: string;
  } | null>(null);

  const openDetail = () => {
    setExpanded(true);
    api.getQualification(lead.id).then(setQual).catch(() => {});
  };

  return (
    <>
      <div onClick={openDetail} className="cursor-pointer rounded-xl border border-[#c3c6d7]/5 bg-white p-3 shadow-sm transition-shadow hover:shadow-md">
        <div className="mb-2 flex items-center justify-between">
          <ChannelBadge channel={lead.channel} />
          <span className="text-[10px] text-[#737686]">{lead.timeAgo}</span>
        </div>
        <p className="text-sm font-bold text-[#0d1c2e]">{lead.name}</p>
        {lead.company && <p className="text-xs text-[#737686]">{lead.company}</p>}
      </div>

      {expanded && (
        <>
          <div className="fixed inset-0 z-40 bg-black/20 backdrop-blur-sm" onClick={() => setExpanded(false)} />
          <div className="fixed inset-y-8 right-8 z-50 w-[min(28rem,calc(100vw-2rem))] overflow-y-auto rounded-2xl bg-white p-6 shadow-2xl">
            <div className="mb-6 flex items-start justify-between">
              <div>
                <ChannelBadge channel={lead.channel} />
                <h3 className="mt-2 text-xl font-extrabold text-[#0d1c2e]">{lead.name}</h3>
                {lead.company && <p className="text-sm text-[#737686]">{lead.company}</p>}
                <p className="mt-1 text-xs text-[#737686]">{lead.timeAgo}</p>
              </div>
              <button onClick={() => setExpanded(false)} className="rounded-lg p-1.5 text-[#434655] hover:bg-[#eff4ff]"><X className="size-5" /></button>
            </div>

            {qual ? (
              <div className="space-y-4">
                <div className="flex items-center gap-3 rounded-xl bg-[#eff4ff] p-4">
                  <div className={`flex size-12 items-center justify-center rounded-full text-lg font-black text-white ${qual.score >= 7 ? "bg-green-500" : qual.score >= 4 ? "bg-amber-500" : "bg-red-500"}`}>{qual.score}</div>
                  <div className="flex-1">
                    <p className="text-xs font-bold uppercase tracking-wider text-[#737686]">Скор квалификации</p>
                    <p className="mt-0.5 text-sm text-[#434655]">{qual.score_reason}</p>
                  </div>
                </div>
                <div className="rounded-xl border border-[#c3c6d7]/10 p-4">
                  <div className="mb-2 flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-[#004ac6]"><Target className="size-3.5" />Потребность</div>
                  <p className="text-sm text-[#434655]">{qual.identified_need}</p>
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div className="rounded-xl border border-[#c3c6d7]/10 p-4">
                    <div className="mb-1 flex items-center gap-1.5 text-xs font-bold uppercase tracking-wider text-[#737686]"><DollarSign className="size-3.5" />Бюджет</div>
                    <p className="text-sm font-medium text-[#0d1c2e]">{qual.estimated_budget}</p>
                  </div>
                  <div className="rounded-xl border border-[#c3c6d7]/10 p-4">
                    <div className="mb-1 flex items-center gap-1.5 text-xs font-bold uppercase tracking-wider text-[#737686]"><Calendar className="size-3.5" />Сроки</div>
                    <p className="text-sm font-medium text-[#0d1c2e]">{qual.deadline}</p>
                  </div>
                </div>
                <div className="rounded-xl bg-[#e1e0ff]/30 p-4">
                  <div className="mb-2 flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-[#3e3fcc]"><Zap className="size-3.5" />Рекомендация</div>
                  <p className="text-sm text-[#434655]">{qual.recommended_action}</p>
                </div>
              </div>
            ) : (
              <div className="py-8 text-center text-sm text-[#737686]">Нет данных квалификации</div>
            )}

            <div className="mt-6 flex gap-2">
              <a href={`/inbox/${lead.id}`} className="flex flex-1 items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-[#004ac6] to-[#2563eb] py-3 text-sm font-bold text-white shadow-md hover:-translate-y-0.5 hover:shadow-lg transition-all">
                Открыть лида
              </a>
            </div>
          </div>
        </>
      )}
    </>
  );
}
