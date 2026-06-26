import Link from "next/link";
import { ArrowUpRight } from "lucide-react";
import { SECTIONS } from "./constants";

// SectionsMap — карта разделов системы: что где находится и зачем. Каждая
// карточка ведёт в реальный раздел.
export function SectionsMap() {
  return (
    <section className="mb-16">
      <h3 className="mb-2 text-xs font-bold uppercase tracking-widest text-[#434655]/50">
        Разделы системы
      </h3>
      <p className="mb-6 text-sm leading-relaxed text-[#434655]/80">
        Куда нажимать и что там. Кликните карточку, чтобы открыть раздел.
      </p>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {SECTIONS.map((s) => (
          <Link
            key={s.href}
            href={s.href}
            className="group flex gap-3 rounded-2xl bg-white p-4 shadow-sm ring-1 ring-[#c3c6d7]/10 transition-all hover:-translate-y-0.5 hover:shadow-md"
          >
            <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-[#eff4ff] text-[#004ac6]">
              <s.icon className="size-[18px]" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="mb-0.5 flex items-center gap-1 text-sm font-bold text-[#0d1c2e]">
                {s.label}
                <ArrowUpRight className="size-3.5 text-[#c3c6d7] transition-colors group-hover:text-[#004ac6]" />
              </div>
              <p className="text-xs leading-relaxed text-[#737686]">{s.what}</p>
            </div>
          </Link>
        ))}
      </div>
    </section>
  );
}
