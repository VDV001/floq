import { GLOSSARY } from "./constants";

// Glossary — ключевые термины простыми словами. Снимает путаницу
// «лид vs проспект», «автопилот vs одобрение» и т.п.
export function Glossary() {
  return (
    <section className="mb-16">
      <h3 className="mb-2 text-xs font-bold uppercase tracking-widest text-[#434655]/50">
        Словарь терминов
      </h3>
      <p className="mb-6 text-sm leading-relaxed text-[#434655]/80">
        Если запутались в словах — здесь коротко и по делу.
      </p>
      <dl className="grid gap-3 sm:grid-cols-2">
        {GLOSSARY.map((t) => (
          <div
            key={t.term}
            className="flex gap-3 rounded-2xl bg-white p-4 shadow-sm ring-1 ring-[#c3c6d7]/10"
          >
            <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-[#eff4ff] text-[#004ac6]">
              <t.icon className="size-[18px]" />
            </div>
            <div>
              <dt className="mb-0.5 text-sm font-bold text-[#0d1c2e]">{t.term}</dt>
              <dd className="text-xs leading-relaxed text-[#737686]">{t.def}</dd>
            </div>
          </div>
        ))}
      </dl>
    </section>
  );
}
