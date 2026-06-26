import Link from "next/link";
import { Check, ArrowRight, ChevronDown, Lightbulb } from "lucide-react";
import { cn } from "@/lib/utils";
import type { UserSettings } from "@/lib/api";
import { STEPS, type Counts, type StepDetails } from "./constants";

interface StepTimelineProps {
  settings: UserSettings | null;
  counts: Counts;
}

// StepGuide раскрывает пошаговый разбор шага. Нативный <details> — доступно с
// клавиатуры и без JS-состояния. Раскрыт по умолчанию для текущего шага.
function StepGuide({ details, defaultOpen }: { details: StepDetails; defaultOpen: boolean }) {
  return (
    <details open={defaultOpen} className="group/d mt-1 rounded-xl border border-[#dbe1ff] bg-[#f7f9ff]">
      <summary className="flex cursor-pointer list-none items-center justify-between gap-2 px-4 py-2.5 text-xs font-bold uppercase tracking-wider text-[#004ac6] [&::-webkit-details-marker]:hidden">
        Подробная инструкция
        <ChevronDown className="size-4 transition-transform group-open/d:rotate-180" />
      </summary>
      <div className="space-y-4 px-4 pb-4 pt-1">
        <p className="text-sm leading-relaxed text-[#434655]">
          <span className="font-bold text-[#0d1c2e]">Зачем: </span>
          {details.why}
        </p>
        <div>
          <div className="mb-2 text-xs font-bold uppercase tracking-wider text-[#434655]/60">
            Как сделать
          </div>
          <ol className="space-y-2">
            {details.how.map((h, idx) => (
              <li key={idx} className="flex gap-3 text-sm leading-relaxed text-[#434655]">
                <span className="flex size-5 shrink-0 items-center justify-center rounded-full bg-[#dbe1ff] text-[11px] font-bold text-[#004ac6]">
                  {idx + 1}
                </span>
                <span>{h}</span>
              </li>
            ))}
          </ol>
        </div>
        <p className="rounded-lg bg-green-50 px-3 py-2 text-sm leading-relaxed text-green-800">
          <span className="font-bold">Что дальше: </span>
          {details.result}
        </p>
        {details.tip && (
          <p className="flex gap-2 text-xs leading-relaxed text-[#737686]">
            <Lightbulb className="size-4 shrink-0 text-[#d97706]" />
            <span>{details.tip}</span>
          </p>
        )}
      </div>
    </details>
  );
}

export function StepTimeline({ settings, counts }: StepTimelineProps) {
  return (
    <section className="relative mb-20">
      {/* Vertical line */}
      <div className="absolute left-[23px] top-6 bottom-6 w-px bg-[#dbe1ff]" />

      <div className="space-y-2">
        {STEPS.map((step, i) => {
          const done = settings
            ? step.check(settings, counts)
            : false;
          const isNext =
            !done &&
            (i === 0 ||
              (settings
                ? STEPS[i - 1].check(settings, counts)
                : false));

          return (
            <div
              key={step.id}
              className={cn(
                "group relative flex gap-5 rounded-2xl p-5 transition-all duration-300",
                done
                  ? "bg-white/40"
                  : isNext
                    ? "bg-white shadow-sm shadow-[#004ac6]/5 ring-1 ring-[#004ac6]/10"
                    : "bg-transparent opacity-60"
              )}
            >
              {/* Step indicator */}
              <div
                className={cn(
                  "relative z-10 flex size-[46px] shrink-0 items-center justify-center rounded-xl transition-all duration-300",
                  done
                    ? "bg-gradient-to-br from-green-400 to-emerald-500 text-white shadow-md shadow-green-500/25"
                    : isNext
                      ? "bg-gradient-to-br from-[#004ac6] to-[#2563eb] text-white shadow-md shadow-[#004ac6]/25"
                      : "bg-[#eff4ff] text-[#434655] ring-1 ring-[#c3c6d7]/20"
                )}
              >
                {done ? (
                  <Check className="size-5" strokeWidth={3} />
                ) : (
                  <step.icon className="size-5" />
                )}
              </div>

              {/* Content */}
              <div className="flex-1 pt-0.5">
                <div className="mb-1 flex items-center gap-3">
                  <h3
                    className={cn(
                      "text-base font-bold",
                      done
                        ? "text-[#434655] line-through decoration-green-400 decoration-2"
                        : "text-[#0d1c2e]"
                    )}
                  >
                    {step.title}
                  </h3>
                  {done && (
                    <span className="rounded-full bg-green-100 px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-wider text-green-700">
                      Готово
                    </span>
                  )}
                </div>
                <p className="mb-3 text-sm leading-relaxed text-[#434655]/80">
                  {step.description}
                </p>

                <StepGuide details={step.details} defaultOpen={isNext} />

                {!done && (
                  <Link
                    href={step.href}
                    className={cn(
                      "mt-3 inline-flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-bold transition-all",
                      isNext
                        ? "bg-gradient-to-r from-[#004ac6] to-[#2563eb] text-white shadow-md shadow-[#004ac6]/20 hover:-translate-y-0.5 hover:shadow-lg"
                        : "bg-[#eff4ff] text-[#004ac6] hover:bg-[#dbe1ff]"
                    )}
                  >
                    {step.btnLabel}
                    <ArrowRight className="size-3.5" />
                  </Link>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}
