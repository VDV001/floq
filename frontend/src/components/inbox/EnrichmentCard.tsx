import { Building2, Mail, Phone, Link2, Briefcase, Users, FileText, MapPin } from "lucide-react";
import type { Enrichment } from "@/lib/api";

interface EnrichmentCardProps {
  enrichment: Enrichment | null;
  loading: boolean;
}

// Human-readable headcount labels for the Phase-2 (#186) company-size buckets.
const COMPANY_SIZE_LABELS: Record<string, string> = {
  solo: "1 сотрудник",
  small: "2–10 сотрудников",
  medium: "11–50 сотрудников",
  large: "51–250 сотрудников",
  enterprise: "250+ сотрудников",
};

function hasLegal(p: Enrichment["profile"]): boolean {
  const l = p.legal;
  return !!l && !!(l.inn || l.ogrn || l.fullName || l.address || l.okved || l.status);
}

function isEmptyProfile(e: Enrichment): boolean {
  const p = e.profile;
  return !p.title && !p.description && p.emails.length === 0 && p.phones.length === 0 &&
    p.socials.length === 0 && !p.industry && !p.companySize && !hasLegal(p);
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
            {(enrichment.profile.industry || enrichment.profile.companySize) && (
              <div className="flex flex-wrap items-center gap-3">
                {enrichment.profile.industry && (
                  <span className="inline-flex items-center gap-1.5 rounded-full bg-white px-3 py-1 text-xs font-medium text-[#434655]">
                    <Briefcase className="size-3.5 text-[#737686]" />
                    {enrichment.profile.industry}
                  </span>
                )}
                {enrichment.profile.companySize && (
                  <span className="inline-flex items-center gap-1.5 rounded-full bg-white px-3 py-1 text-xs font-medium text-[#434655]">
                    <Users className="size-3.5 text-[#737686]" />
                    {COMPANY_SIZE_LABELS[enrichment.profile.companySize] ?? enrichment.profile.companySize}
                  </span>
                )}
              </div>
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
            {hasLegal(enrichment.profile) && enrichment.profile.legal && (
              <div className="mt-2 space-y-1.5 rounded-lg bg-white/70 p-3">
                <div className="flex items-center gap-1.5 text-xs font-semibold text-[#0d1c2e]">
                  <FileText className="size-3.5 text-[#737686]" />
                  Реквизиты
                </div>
                {enrichment.profile.legal.fullName && (
                  <p className="text-sm text-[#0d1c2e]">{enrichment.profile.legal.fullName}</p>
                )}
                <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-[#434655]">
                  {enrichment.profile.legal.inn && <span>ИНН {enrichment.profile.legal.inn}</span>}
                  {enrichment.profile.legal.ogrn && <span>ОГРН {enrichment.profile.legal.ogrn}</span>}
                  {enrichment.profile.legal.okved && <span>ОКВЭД {enrichment.profile.legal.okved}</span>}
                  {enrichment.profile.legal.status && <span>{enrichment.profile.legal.status}</span>}
                </div>
                {enrichment.profile.legal.address && (
                  <div className="flex items-center gap-1.5 text-xs text-[#434655]">
                    <MapPin className="size-3.5 text-[#737686]" />
                    {enrichment.profile.legal.address}
                  </div>
                )}
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
