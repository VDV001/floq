import type { Prospect } from "@/lib/api";

// Minimal valid Prospect for integration fixtures; override per test.
export function prospect(over: Partial<Prospect> = {}): Prospect {
  return {
    id: over.id ?? "p-1",
    user_id: over.user_id ?? "u-1",
    name: over.name ?? "Иван Петров",
    company: over.company ?? "Acme",
    title: over.title ?? "CEO",
    email: over.email ?? "ivan@acme.io",
    phone: over.phone ?? "",
    whatsapp: over.whatsapp ?? "",
    telegram_username: over.telegram_username ?? "",
    industry: over.industry ?? "",
    company_size: over.company_size ?? "",
    context: over.context ?? "",
    source: over.source ?? "csv",
    source_id: over.source_id,
    source_name: over.source_name ?? "Источник A",
    status: over.status ?? "new",
    consent_status: over.consent_status ?? "none",
    consent_source: over.consent_source,
    verify_status: over.verify_status ?? "not_checked",
    verify_score: over.verify_score ?? 0,
    verify_details: over.verify_details ?? {},
    verified_at: over.verified_at ?? null,
    converted_lead_id: over.converted_lead_id ?? null,
    created_at: over.created_at ?? "2026-06-01T00:00:00Z",
    updated_at: over.updated_at ?? "2026-06-01T00:00:00Z",
  };
}
