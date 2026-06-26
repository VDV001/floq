# Auto-enrichment

Background enrichment of a lead/prospect's **company** from public sources,
keyed by the domain of their email. Lives in the `internal/enrichment` bounded
context. Shipped in three phases, each behind its own flag.

## What the user sees
A "О компании" card on the lead/inbox screen (`EnrichmentCard`): title,
description, contacts, socials (phase 1); industry + company size (phase 2);
legal requisites — ИНН/ОГРН/адрес/ОКВЭД/статус (phase 3).

Read API: `GET /api/enrichment?email=<addr>` (tenant-scoped; the domain is
derived from the email server-side). Free/personal email providers and
malformed addresses return `status:"none"`.

## Pipeline
A table-as-queue (`company_enrichment`, per `(user_id, domain)`) doubles as
cache + dedup + retry. `EnrichmentCron` ticks every
`ENRICHMENT_REFRESH_INTERVAL`, claims due rows (pending / failed-under-cap /
enriched-and-expired), and for each:

```
rate-limit (per domain) → fetch website → extract → [registry enrich] → save
```

Every step is best-effort and per-row isolated: one domain's failure (or panic)
never aborts the batch, and a later step's failure never discards an earlier
step's data.

## Phase 1 — website scraping (#182, v0.55.0)
`WebsiteFetcher` (egress-guarded HTTP client) fetches the company homepage;
`HTMLExtractor` pulls title/description/emails/phones/socials by regex — no LLM.

**Security:** the fetch URL is derived from an attacker-influenceable email
domain, so SSRF is defended in two layers: (1) `domain.NewDomain` rejects IP
literals and `host:port`; (2) the client's dial guard blocks loopback / private
/ link-local / metadata IPs on the **resolved** address and re-checks every
redirect hop. The client is always direct (ignores `PROXY_URL`) so the guard
always sees the real target IP.

## Phase 2 — LLM industry/size (#186, v0.56.0)
`ChainExtractor` wraps the HTML extractor behind the same `Extractor` port
(open/closed): HTML stays the deterministic base; `LLMExtractor` overlays
`industry` + `company_size` from the same page. LLM failure is swallowed
(graceful degrade → HTML profile is still saved).

- `CompanySize` is a typed enum (solo/small/medium/large/enterprise); `Industry`
  is a normalized free-text VO. Both live inside the `CompanyProfile` JSONB — no
  schema migration.
- The LLM call goes through a cost-capped adapter over the **audit-recording**
  provider: Budget model mode, small `MaxTokens`, input rune-cap, and
  `request_type='enrichment'` so spend is attributed (see `audit-log.md`). The
  worker attaches a subject-user to the context so a fresh `CallMeta` is built —
  `WithRequestType` alone is a no-op without a parent meta and the audit row
  would be dropped.

Flags: `ENRICHMENT_LLM_ENABLED` (default off), `ENRICHMENT_LLM_MAX_INPUT_RUNES`,
`ENRICHMENT_LLM_MAX_TOKENS`.

## Phase 3 — legal requisites via DaData (#188, v0.57.0)
A new `Enricher` port (orthogonal to `Extractor`: it reaches out to a registry
by company identity). The `daDataEnricher` adapter (composition root) calls the
official **DaData** party API and merges `LegalDetails` (ИНН/ОГРН/full name/
address/ОКВЭД/status) into the profile.

**Matching is honest — it never guesses:**
1. If an ИНН appears on the page (label-gated regex + checksum via `domain.NewINN`),
   look it up precisely with `findById`.
2. Otherwise fuzzy-match the scraped company name with `suggest`, and accept the
   result **only when it is a single unique hit**. Any ambiguous (2+) result is
   a miss — a wrong ИНН is worse than none.

`INN`/`OGRN` are value objects with control-digit validation (10/12 and 13/15
digits). `LegalDetails` lives in the `CompanyProfile` JSONB — no migration.

**Security:** DaData is a constant, trusted host; the company name / ИНН travel
in the POST body, never in the URL — so there is **no SSRF surface** (unlike
phase 1) and a standard HTTP client suffices. The API key is sent only in the
`Authorization: Token` header and never logged. A global rate limit
(`ENRICHMENT_REGISTRY_RATE_LIMIT_PER_MIN`) protects the shared daily quota.

Flags: `ENRICHMENT_REGISTRY_ENABLED` (default off) + `DADATA_API_KEY` (empty key
disables the enricher even if the flag is on — ship-dark).

Backlog: a Prometheus calls/hits counter for DaData visibility once enabled.

## Configuration
All `ENRICHMENT_*` / `DADATA_API_KEY` variables are listed in `.env.example`.
Defaults keep phases 2 and 3 **off** — enable them per environment after
validation.
