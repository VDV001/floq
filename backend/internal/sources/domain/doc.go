// Package domain models the Sources bounded context — the taxonomy by which
// leads and prospects are attributed ("where did this contact come from?").
// Two-level hierarchy: Category → Source.
//
// Ubiquitous language
//
//   - Category    top-level grouping (e.g. "Cold outbound", "Referrals",
//                 "Business clubs").
//   - Source      concrete source within a category (e.g. "2GIS",
//                 "BK Magnat", "LinkedIn").
//   - SourceStat  read model: per-source counts (prospect_count,
//                 lead_count, converted_count) used by the analytics
//                 widget on the prospects page.
//
// Invariants
//
//   - Factories (NewCategory, NewSource) validate non-empty names.
//   - Unique constraints live at the DB level (migration 022) to guard
//     against duplicate categories per user / duplicate sources per
//     category.
//
// Design notes
//
//   - SourceStat is the canonical example of "read model kept separate
//     from the aggregate" — see prospects.ProspectWithSource and
//     leads.LeadWithSource for the same pattern applied to list projections.
package domain
