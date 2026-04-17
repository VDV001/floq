import { Brain, Sparkles, CheckCircle2 } from "lucide-react";
import type { Qualification } from "@/lib/api";

interface QualificationCardProps {
  qualification: Qualification | null;
  loading: boolean;
}

export function QualificationCard({ qualification, loading }: QualificationCardProps) {
  return (
    <section className="mb-10">
      <div className="relative overflow-hidden rounded-xl border border-[#c0c1ff]/20 bg-[#e1e0ff]/30 p-6">
        <div className="absolute right-0 top-0 p-8 opacity-5"><Sparkles className="size-24" /></div>

        <div className="relative z-10 mb-6 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Brain className="size-5 text-[#3e3fcc]" />
            <h3 className="text-lg font-bold text-[#07006c]">ИИ-квалификация лида</h3>
          </div>
          {qualification && (
            <div className="flex items-center gap-3">
              <span className="text-xs font-bold uppercase tracking-wider text-[#2f2ebe]">Оценка</span>
              <div className="relative flex size-14 items-center justify-center rounded-full border-4 border-[#585be6]/30">
                <span className="text-sm font-extrabold text-[#3e3fcc]">{qualification.score}</span>
              </div>
            </div>
          )}
        </div>

        {loading ? (
          <div className="relative z-10 flex items-center gap-2 text-sm text-[#2f2ebe]">
            <div className="size-4 animate-spin rounded-full border-2 border-[#3e3fcc] border-t-transparent" />
            Загрузка квалификации...
          </div>
        ) : qualification ? (
          <>
            <div className="relative z-10 grid grid-cols-3 gap-6">
              <div className="rounded-lg bg-white/60 p-4 backdrop-blur-sm">
                <p className="mb-1 text-[0.65rem] font-bold uppercase text-[#2f2ebe]">Выявленная потребность</p>
                <p className="text-sm font-medium leading-relaxed text-[#0d1c2e]">{qualification.identified_need}</p>
              </div>
              <div className="rounded-lg bg-white/60 p-4 backdrop-blur-sm">
                <p className="mb-1 text-[0.65rem] font-bold uppercase text-[#2f2ebe]">Оценка бюджета</p>
                <p className="text-sm font-medium text-[#0d1c2e]">{qualification.estimated_budget}</p>
              </div>
              <div className="rounded-lg bg-white/60 p-4 backdrop-blur-sm">
                <p className="mb-1 text-[0.65rem] font-bold uppercase text-[#2f2ebe]">Сроки</p>
                <p className="text-sm font-medium text-[#0d1c2e]">{qualification.deadline}</p>
              </div>
            </div>
            <div className="relative z-10 mt-6 flex w-fit items-center gap-2 rounded-full bg-[#c0c1ff]/40 px-4 py-2 text-xs font-semibold text-[#3e3fcc]">
              <CheckCircle2 className="size-4" />
              {qualification.recommended_action}
            </div>
          </>
        ) : (
          <p className="relative z-10 text-sm italic text-[#2f2ebe]/70">Ожидает квалификации ИИ...</p>
        )}
      </div>
    </section>
  );
}
