import Link from "next/link";
import { ChevronDown, ArrowRight, LifeBuoy } from "lucide-react";
import { FAQ } from "./constants";

// Faq — секция «Если что-то не работает»: частые сбои новичка с разбором.
// Нативный <details> — доступно с клавиатуры и без JS-состояния (все свёрнуты).
export function Faq() {
  return (
    <section className="mb-16">
      <h3 className="mb-2 flex items-center gap-2 text-xs font-bold uppercase tracking-widest text-[#434655]/50">
        <LifeBuoy className="size-4 text-[#d97706]" />
        Если что-то не работает
      </h3>
      <p className="mb-6 text-sm leading-relaxed text-[#434655]/80">
        Частые ситуации «сделал по инструкции, но не сработало» — и что проверить.
      </p>
      <div className="space-y-2">
        {FAQ.map((item) => (
          <details
            key={item.q}
            className="group/f rounded-xl border border-[#dbe1ff] bg-white"
          >
            <summary className="flex cursor-pointer list-none items-center justify-between gap-3 px-4 py-3 text-sm font-bold text-[#0d1c2e] [&::-webkit-details-marker]:hidden">
              {item.q}
              <ChevronDown className="size-4 shrink-0 text-[#004ac6] transition-transform group-open/f:rotate-180" />
            </summary>
            <div className="space-y-3 px-4 pb-4 pt-1">
              <div className="text-xs font-bold uppercase tracking-wider text-[#434655]/60">
                Решение
              </div>
              <ol className="space-y-2">
                {item.a.map((step, idx) => (
                  <li
                    key={idx}
                    className="flex gap-3 text-sm leading-relaxed text-[#434655]"
                  >
                    <span className="flex size-5 shrink-0 items-center justify-center rounded-full bg-[#dbe1ff] text-[11px] font-bold text-[#004ac6]">
                      {idx + 1}
                    </span>
                    <span>{step}</span>
                  </li>
                ))}
              </ol>
              <Link
                href={item.href}
                className="inline-flex items-center gap-1.5 rounded-lg bg-[#eff4ff] px-3 py-1.5 text-xs font-bold text-[#004ac6] transition-colors hover:bg-[#dbe1ff]"
              >
                {item.hrefLabel}
                <ArrowRight className="size-3" />
              </Link>
            </div>
          </details>
        ))}
      </div>
    </section>
  );
}
