import { cn } from "@/lib/utils";
import { ArrowDown } from "lucide-react";
import { HOW_IT_WORKS, PIPELINE_STAGES } from "./constants";

// HowItWorks объясняет систему до настройки: два «движка» (входящий и
// исходящий) и стадии воронки, по которым движется лид.
export function HowItWorks() {
  return (
    <section className="mb-16">
      <h3 className="mb-2 text-xs font-bold uppercase tracking-widest text-[#434655]/50">
        Как работает Floq
      </h3>
      <p className="mb-6 text-sm leading-relaxed text-[#434655]/80">
        Floq — это два потока в одном месте. Клиенты пишут вам сами (входящий) и
        вы пишете первыми по холодной базе (исходящий). Оба сходятся в едином
        списке лидов, где AI помогает на каждом шаге.
      </p>

      <div className="grid gap-4 sm:grid-cols-2">
        {HOW_IT_WORKS.map((engine) => (
          <div
            key={engine.id}
            className="rounded-2xl bg-white p-6 shadow-sm ring-1 ring-[#c3c6d7]/10"
          >
            <div className="mb-4 flex items-center gap-3">
              <div
                className={cn(
                  "flex size-10 items-center justify-center rounded-lg bg-gradient-to-br text-white",
                  engine.accent
                )}
              >
                <engine.icon className="size-5" />
              </div>
              <div>
                <h4 className="text-sm font-bold text-[#0d1c2e]">{engine.title}</h4>
                <p className="text-xs text-[#737686]">{engine.subtitle}</p>
              </div>
            </div>
            <ol className="space-y-2.5">
              {engine.steps.map((s, idx) => (
                <li key={idx} className="flex gap-3 text-sm leading-relaxed text-[#434655]">
                  <span className="flex size-5 shrink-0 items-center justify-center rounded-full bg-[#eff4ff] text-[11px] font-bold text-[#004ac6]">
                    {idx + 1}
                  </span>
                  <span>{s}</span>
                </li>
              ))}
            </ol>
          </div>
        ))}
      </div>

      {/* Pipeline */}
      <div className="mt-6 rounded-2xl bg-white p-6 shadow-sm ring-1 ring-[#c3c6d7]/10">
        <h4 className="mb-4 text-sm font-bold text-[#0d1c2e]">
          Путь лида по воронке
        </h4>
        <div className="flex flex-col gap-2 sm:flex-row sm:items-stretch sm:gap-0">
          {PIPELINE_STAGES.map((stage, idx) => (
            <div key={stage.label} className="flex flex-1 items-center gap-2 sm:flex-col sm:gap-2">
              <div className="flex-1 rounded-xl bg-[#f7f9ff] px-3 py-2.5 text-center ring-1 ring-[#dbe1ff] sm:w-full">
                <div className="text-sm font-bold text-[#0d1c2e]">{stage.label}</div>
                <div className="mt-0.5 text-[11px] leading-snug text-[#737686]">{stage.desc}</div>
              </div>
              {idx < PIPELINE_STAGES.length - 1 && (
                <ArrowDown className="size-4 shrink-0 text-[#c3c6d7] sm:rotate-[-90deg]" />
              )}
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
