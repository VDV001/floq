import { Building2, Mail, Phone, Link2 } from "lucide-react";
import type { Enrichment } from "@/lib/api";

interface EnrichmentCardProps {
  enrichment: Enrichment | null;
  loading: boolean;
}

function isEmptyProfile(e: Enrichment): boolean {
  const p = e.profile;
  return !p.title && !p.description && p.emails.length === 0 && p.phones.length === 0 && p.socials.length === 0;
}

export function EnrichmentCard({ enrichment, loading }: EnrichmentCardProps) {
  const hasData = enrichment != null && enrichment.status === "enriched" && !isEmptyProfile(enrichment);
  const pending = enrichment != null && enrichment.status === "pending";

  return (
    <section className="mb-10">
      <div className="relative overflow-hidden rounded-xl border border-[#c0c1ff]/20 bg-[#eff4ff]/60 p-6">
        <div className="relative z-10 mb-4 flex items-center gap-2">
          <Building2 className="size-5 text-[#004ac6]" />
          <h3 className="text-lg font-bold text-[#0d1c2e]">О компании</h3>
        </div>

        {loading ? (
          <div className="relative z-10 flex items-center gap-2 text-sm text-[#434655]">
            <div className="size-4 animate-spin rounded-full border-2 border-[#004ac6] border-t-transparent" />
            Загрузка данных о компании...
          </div>
        ) : hasData && enrichment ? (
          <div className="relative z-10 space-y-3">
            {enrichment.profile.title && (
              <p className="text-base font-bold text-[#0d1c2e]">{enrichment.profile.title}</p>
            )}
            {enrichment.profile.description && (
              <p className="text-sm leading-relaxed text-[#434655]">{enrichment.profile.description}</p>
            )}
            {enrichment.profile.emails.length > 0 && (
              <div className="flex flex-wrap items-center gap-2">
                <Mail className="size-4 text-[#737686]" />
                {enrichment.profile.emails.map((e) => (
                  <a key={e} href={`mailto:${e}`} className="rounded-full bg-white px-3 py-1 text-xs font-medium text-[#004ac6] hover:underline">{e}</a>
                ))}
              </div>
            )}
            {enrichment.profile.phones.length > 0 && (
              <div className="flex flex-wrap items-center gap-2">
                <Phone className="size-4 text-[#737686]" />
                {enrichment.profile.phones.map((p) => (
                  <a key={p} href={`tel:${p}`} className="rounded-full bg-white px-3 py-1 text-xs font-medium text-[#004ac6] hover:underline">{p}</a>
                ))}
              </div>
            )}
            {enrichment.profile.socials.length > 0 && (
              <div className="flex flex-wrap items-center gap-2">
                <Link2 className="size-4 text-[#737686]" />
                {enrichment.profile.socials.map((s) => (
                  <a key={s} href={s} target="_blank" rel="noopener noreferrer" className="rounded-full bg-white px-3 py-1 text-xs font-medium text-[#004ac6] hover:underline">{s.replace(/^https?:\/\//, "")}</a>
                ))}
              </div>
            )}
            <p className="pt-1 text-[0.65rem] uppercase tracking-wider text-[#737686]">
              {enrichment.domain}
            </p>
          </div>
        ) : pending ? (
          <p className="relative z-10 text-sm italic text-[#434655]/70">Собираем данные о компании...</p>
        ) : (
          <p className="relative z-10 text-sm italic text-[#737686]">Нет данных о компании</p>
        )}
      </div>
    </section>
  );
}
