import { cn } from "@/lib/utils";
import { TIPS } from "./constants";

export function TipsSection() {
  return (
    <section className="mb-16">
      <h3 className="mb-6 text-xs font-bold uppercase tracking-widest text-[#434655]/50">
        Полезные возможности
      </h3>
      <div className="grid gap-4 sm:grid-cols-3">
        {TIPS.map((tip) => (
          <div
            key={tip.title}
            className="group rounded-2xl bg-white p-6 shadow-sm ring-1 ring-[#c3c6d7]/10 transition-all duration-300 hover:-translate-y-0.5 hover:shadow-md"
          >
            <div
              className={cn(
                "mb-4 flex size-10 items-center justify-center rounded-lg bg-gradient-to-br text-white",
                tip.accent
              )}
            >
              <tip.icon className="size-5" />
            </div>
            <h4 className="mb-2 text-sm font-bold text-[#0d1c2e]">
              {tip.title}
            </h4>
            <p className="text-xs leading-relaxed text-[#434655]/80">
              {tip.description}
            </p>
          </div>
        ))}
      </div>
    </section>
  );
}
