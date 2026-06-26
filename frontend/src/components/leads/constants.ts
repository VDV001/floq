// Lead-domain primitives. Owned by components/leads/ so view-layer
// surfaces (inbox, pipeline, dashboard widgets) can render a lead
// without depending on the view that first happened to define the
// shared styles.
//
// Inbox-specific things (PIPELINE_STAGES_CONFIG, FILTER_TABS,
// mapStatus, InboxLead) stay in components/inbox/constants.ts.

export type LeadStatus =
  | "Новый"
  | "Квалифицирован"
  | "В диалоге"
  | "Нужен фоллоуап"
  | "Закрыт"
  | "Выигран";

export const STATUS_STYLES: Record<LeadStatus, string> = {
  "Новый": "bg-[#dbe1ff] text-[#004ac6]",
  "Квалифицирован": "bg-[#c7d2fe] text-[#3730a3]",
  "В диалоге": "bg-[#fef3c7] text-[#92400e]",
  "Нужен фоллоуап": "bg-[#fee2e2] text-[#dc2626]",
  "Закрыт": "bg-[#d1fae5] text-[#065f46]",
  "Выигран": "bg-[#bbf7d0] text-[#14532d]",
};
