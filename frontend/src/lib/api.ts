const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// ApiError carries the backend's human-readable message plus an optional
// machine `code` and `remedy` ("what to do") so the UI can show the real cause
// of a failure instead of a generic "API error: 500".
export class ApiError extends Error {
  readonly status: number;
  readonly code?: string;
  readonly remedy?: string;
  constructor(message: string, status: number, code?: string, remedy?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.remedy = remedy;
  }
}

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const token =
    typeof window !== "undefined" ? localStorage.getItem("token") : null;

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options?.headers,
    },
  });

  if (res.status === 401 && typeof window !== "undefined") {
    // Try to refresh token
    const refreshToken = localStorage.getItem("refresh_token");
    if (refreshToken) {
      try {
        const refreshRes = await fetch(`${API_BASE}/api/auth/refresh`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ refresh_token: refreshToken }),
        });
        if (refreshRes.ok) {
          const data = await refreshRes.json();
          localStorage.setItem("token", data.token);
          localStorage.setItem("refresh_token", data.refresh_token);
          // Retry original request with new token
          const retryRes = await fetch(`${API_BASE}${path}`, {
            ...options,
            headers: {
              "Content-Type": "application/json",
              Authorization: `Bearer ${data.token}`,
              ...options?.headers,
            },
          });
          if (retryRes.ok) {
            if (retryRes.status === 204) return undefined as T;
            return retryRes.json();
          }
        }
      } catch {
        // refresh failed
      }
      localStorage.removeItem("token");
      localStorage.removeItem("refresh_token");
      window.location.href = "/login";
    }
  }

  if (!res.ok) {
    // Read the backend error envelope ({error, code, remedy}) so the real
    // cause and the suggested fix reach the user. Fall back to the status
    // line when the body is empty or not JSON.
    let body: { error?: string; code?: string; remedy?: string } | null = null;
    try {
      body = await res.json();
    } catch {
      // non-JSON or empty body — keep the status-line fallback
    }
    const message = body?.error || `${res.status} ${res.statusText}`;
    throw new ApiError(message, res.status, body?.code, body?.remedy);
  }

  // 204 No Content has an empty body — calling .json() throws
  // SyntaxError. The HITL approve/reject endpoints answer 204 on
  // success; callers expect undefined (the generic T is typically
  // void at the call site).
  if (res.status === 204) return undefined as T;
  return res.json();
}

async function apiDownload(path: string): Promise<void> {
  const token =
    typeof window !== "undefined" ? localStorage.getItem("token") : null;
  const res = await fetch(`${API_BASE}${path}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  if (!res.ok) throw new Error(`Download error: ${res.status}`);
  const blob = await res.blob();
  const disposition = res.headers.get("Content-Disposition") || "";
  const match = disposition.match(/filename="?([^"]+)"?/);
  const filename = match?.[1] || "export.csv";
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

async function apiUploadFile<T>(path: string, file: File): Promise<T> {
  const token =
    typeof window !== "undefined" ? localStorage.getItem("token") : null;
  const formData = new FormData();
  formData.append("file", file);
  const res = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    body: formData,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => null);
    throw new Error(body?.error || `Upload error: ${res.status}`);
  }
  return res.json();
}

export const api = {
  // Auth
  register: (email: string, password: string, fullName: string) =>
    apiFetch<{ token: string; refresh_token: string }>("/api/auth/register", {
      method: "POST",
      body: JSON.stringify({ email, password, full_name: fullName }),
    }),

  login: (email: string, password: string) =>
    apiFetch<{ token: string; refresh_token: string }>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    }),

  refresh: (refreshToken: string) =>
    apiFetch<{ token: string; refresh_token: string }>("/api/auth/refresh", {
      method: "POST",
      body: JSON.stringify({ refresh_token: refreshToken }),
    }),

  // Enrichment (#182) — company data scraped by the sender's email domain.
  getEnrichment: (email: string) =>
    apiFetch<Enrichment>(`/api/enrichment?email=${encodeURIComponent(email)}`),

  // Leads
  getLeads: () => apiFetch<Lead[]>("/api/leads"),
  getArchivedLeads: () => apiFetch<Lead[]>("/api/leads/archived"),
  getLead: (id: string) => apiFetch<Lead>(`/api/leads/${id}`),
  updateLeadStatus: (id: string, status: string) =>
    apiFetch(`/api/leads/${id}/status`, {
      method: "PATCH",
      body: JSON.stringify({ status }),
    }),
  archiveLead: (id: string) =>
    apiFetch(`/api/leads/${id}/archive`, { method: "POST" }),
  unarchiveLead: (id: string) =>
    apiFetch(`/api/leads/${id}/unarchive`, { method: "POST" }),

  exportLeadsCSV: () => apiDownload("/api/leads/export"),
  importLeadsCSV: (file: File) =>
    apiUploadFile<{ imported: number }>("/api/leads/import", file),

  // Prospect suggestions (cross-channel dedup)
  getProspectSuggestions: (leadId: string) =>
    apiFetch<ProspectSuggestion[]>(`/api/leads/${leadId}/prospect-suggestions`),
  linkProspect: (leadId: string, prospectId: string) =>
    apiFetch(`/api/leads/${leadId}/link-prospect`, {
      method: "POST",
      body: JSON.stringify({ prospect_id: prospectId }),
    }),
  dismissProspectSuggestion: (leadId: string, prospectId: string) =>
    apiFetch(`/api/leads/${leadId}/dismiss-prospect-suggestion`, {
      method: "POST",
      body: JSON.stringify({ prospect_id: prospectId }),
    }),
  getSuggestionCounts: () =>
    apiFetch<Record<string, number>>("/api/leads/suggestion-counts"),

  // Messages
  //
  // When `aggregated` is true the backend merges messages from every
  // lead sharing the same Identity (multi-source dedup, #27). Default
  // is single-lead — backward-compatible with callers that don't pass
  // the flag.
  getMessages: (leadId: string, opts?: { aggregated?: boolean }) =>
    apiFetch<Message[]>(
      `/api/leads/${leadId}/messages${opts?.aggregated ? "?aggregated=true" : ""}`
    ),
  sendMessage: (leadId: string, body: string) =>
    apiFetch<Message>(`/api/leads/${leadId}/send`, {
      method: "POST",
      body: JSON.stringify({ body }),
    }),

  // Qualification
  getQualification: (leadId: string) =>
    apiFetch<Qualification>(`/api/leads/${leadId}/qualification`),
  qualifyLead: (leadId: string) =>
    apiFetch<Qualification>(`/api/leads/${leadId}/qualify`, { method: "POST" }),

  // Draft
  getDraft: (leadId: string) =>
    apiFetch<Draft>(`/api/leads/${leadId}/draft`),
  regenerateDraft: (leadId: string) =>
    apiFetch<Draft>(`/api/leads/${leadId}/draft/regen`, { method: "POST" }),

  // Pending replies (HITL approval queue)
  //
  // The inbox flow parks auto-drafted replies that would otherwise
  // reach the customer (currently the booking-link branch in the
  // Telegram bot) until an operator approves them. List per lead,
  // approve to dispatch and mark sent, reject to terminate the
  // draft. Approve / reject return 204; status flips are visible by
  // re-listing.
  getPendingReplies: (leadId: string) =>
    apiFetch<PendingReply[]>(`/api/leads/${leadId}/pending-replies`),
  // listPendingReplies — operator queue: every pending row across
  // every lead the operator owns, with the joined lead snippet so the
  // page renders contact + company in one request (no N+1). The
  // status filter is explicit on the wire — keeps room for a future
  // ?status=approved tab without silently widening the contract.
  listPendingReplies: () =>
    apiFetch<PendingReplyQueueRow[]>("/api/pending-replies?status=pending"),
  // bulkPendingReplies — power-operator endpoint: apply the same
  // decision to many drafts in one round-trip. Per-row outcomes come
  // back under `results` so the UI can surface partial failures
  // (NotFound / AlreadyDecided / dispatcher 5xx) without aborting the
  // whole batch.
  bulkPendingReplies: (body: { ids: string[]; decision: PendingReplyBulkDecision }) =>
    apiFetch<{ results: PendingReplyBulkResult[] }>("/api/pending-replies/bulk", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  approvePendingReply: (id: string) =>
    apiFetch<void>(`/api/pending-replies/${id}/approve`, { method: "POST" }),
  rejectPendingReply: (id: string) =>
    apiFetch<void>(`/api/pending-replies/${id}/reject`, { method: "POST" }),
  // Edit-before-approve (#48). Returns the updated PendingReply so the
  // caller can render the new body in place without a refetch round-trip.
  // Only valid while the row is in 'pending' status — backend answers
  // 409 once approved/sent/rejected.
  updatePendingReply: (id: string, body: string) =>
    apiFetch<PendingReply>(`/api/pending-replies/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ body }),
    }),

  // Reminders
  getReminders: () => apiFetch<Reminder[]>("/api/reminders"),
  snoozeReminder: (id: string) =>
    apiFetch(`/api/reminders/${id}/snooze`, { method: "POST" }),
  dismissReminder: (id: string) =>
    apiFetch(`/api/reminders/${id}/dismiss`, { method: "POST" }),

  // Sources
  getSources: () => apiFetch<SourceCategory[]>("/api/sources"),
  createSourceCategory: (name: string) =>
    apiFetch<SourceCategory>("/api/sources/categories", {
      method: "POST",
      body: JSON.stringify({ name }),
    }),
  createSource: (categoryId: string, name: string) =>
    apiFetch<SourceItem>("/api/sources", {
      method: "POST",
      body: JSON.stringify({ category_id: categoryId, name }),
    }),
  getSourceStats: () => apiFetch<SourceStatItem[]>("/api/sources/stats"),

  // Prospects
  getProspects: () => apiFetch<Prospect[]>("/api/prospects"),
  createProspect: (data: CreateProspectBody) =>
    apiFetch<Prospect>("/api/prospects", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  deleteProspect: (id: string) =>
    apiFetch(`/api/prospects/${id}`, { method: "DELETE" }),
  setProspectConsent: (id: string, status: "obtained" | "withdrawn") =>
    apiFetch<{ consent_status: string }>(`/api/prospects/${id}/consent`, {
      method: "POST",
      body: JSON.stringify({ status }),
    }),
  exportProspectsCSV: () => apiDownload("/api/prospects/export"),
  downloadProspectTemplate: () => apiDownload("/api/prospects/template"),
  importProspectsCSV: (file: File) =>
    apiUploadFile<{ imported: number }>("/api/prospects/import", file),

  // Verification
  verifyEmail: (email: string) =>
    apiFetch<EmailVerifyResult>("/api/verify/email", {
      method: "POST",
      body: JSON.stringify({ email }),
    }),
  verifyBatch: () =>
    apiFetch<{ verified: number }>("/api/verify/batch", { method: "POST" }),
  getVerifyStatus: (prospectId: string) =>
    apiFetch<VerifyStatus>(`/api/prospects/${prospectId}/verify`),

  // Parser
  scrapeWebsite: (url: string) =>
    apiFetch<{ url: string; emails: string[] }>("/api/parser/website", {
      method: "POST",
      body: JSON.stringify({ url }),
    }),
  searchTwoGIS: (query: string, city: string) =>
    apiFetch<{ name: string; address: string; phone: string; category: string; website: string; city: string }[]>("/api/parser/twogis", {
      method: "POST",
      body: JSON.stringify({ query, city }),
    }),

  // Sequences
  getSequences: () => apiFetch<Sequence[]>("/api/sequences"),
  getSequence: (id: string) => apiFetch<{ sequence: Sequence; steps: SequenceStep[] }>(`/api/sequences/${id}`),
  createSequence: (name: string) =>
    apiFetch<Sequence>("/api/sequences", { method: "POST", body: JSON.stringify({ name }) }),
  updateSequence: (id: string, name: string) =>
    apiFetch<Sequence>(`/api/sequences/${id}`, { method: "PUT", body: JSON.stringify({ name }) }),
  deleteSequence: (id: string) =>
    apiFetch(`/api/sequences/${id}`, { method: "DELETE" }),
  addStep: (seqId: string, data: { step_order: number; delay_days: number; channel: string; prompt_hint: string; body?: string }) =>
    apiFetch<SequenceStep>(`/api/sequences/${seqId}/steps`, { method: "POST", body: JSON.stringify(data) }),
  deleteStep: (seqId: string, stepId: string) =>
    apiFetch(`/api/sequences/${seqId}/steps/${stepId}`, { method: "DELETE" }),
  previewMessage: (name: string, company: string, context: string, channel: string, hint: string) =>
    apiFetch<{ text: string }>("/api/sequences/preview", {
      method: "POST",
      body: JSON.stringify({ name, company, context, channel, hint }),
    }),
  launchSequence: (seqId: string, prospectIds: string[], sendNow = true) =>
    apiFetch(`/api/sequences/${seqId}/launch`, { method: "POST", body: JSON.stringify({ prospect_ids: prospectIds, send_now: sendNow }) }),
  toggleSequence: (seqId: string, isActive: boolean) =>
    apiFetch(`/api/sequences/${seqId}/toggle`, { method: "PATCH", body: JSON.stringify({ is_active: isActive }) }),

  // Outbound
  getOutboundQueue: () => apiFetch<OutboundMessage[]>("/api/outbound/queue"),
  approveMessage: (id: string) =>
    apiFetch(`/api/outbound/${id}/approve`, { method: "POST" }),
  rejectMessage: (id: string) =>
    apiFetch(`/api/outbound/${id}/reject`, { method: "POST" }),
  editMessage: (id: string, body: string) =>
    apiFetch(`/api/outbound/${id}/edit`, { method: "POST", body: JSON.stringify({ body }) }),
  getOutboundSent: () => apiFetch<OutboundMessage[]>("/api/outbound/sent"),
  getOutboundStats: () => apiFetch<OutboundStats>("/api/outbound/stats"),

  // AI Chat
  chatWithAI: (message: string, history: { role: string; content: string }[], context?: string) =>
    apiFetch<{ reply: string }>("/api/chat", {
      method: "POST",
      body: JSON.stringify({ message, history, context: context || "" }),
    }),

  // Telegram Account (MTProto)
  tgAccountSendCode: (phone: string) =>
    apiFetch<{ code_hash: string }>("/api/telegram-account/send-code", {
      method: "POST",
      body: JSON.stringify({ phone }),
    }),
  tgAccountVerify: (phone: string, code: string, codeHash: string) =>
    apiFetch<{ status: string }>("/api/telegram-account/verify", {
      method: "POST",
      body: JSON.stringify({ phone, code, code_hash: codeHash }),
    }),
  tgAccountStatus: () =>
    apiFetch<{ connected: boolean; phone: string }>("/api/telegram-account/status"),
  tgAccountDisconnect: () =>
    apiFetch("/api/telegram-account", { method: "DELETE" }),

  // Usage
  getUsage: () => apiFetch<{ plan: string; limit: number; month_leads: number; total_leads: number }>("/api/usage"),

  // Settings
  getSettings: () => apiFetch<UserSettings>("/api/settings"),
  updateSettings: (data: Partial<UserSettings>) =>
    apiFetch<UserSettings>("/api/settings", {
      method: "PUT",
      body: JSON.stringify(data),
    }),
  testIMAP: (host: string, port: string, user: string, password: string, useStored?: boolean) =>
    apiFetch<{ success: boolean; message?: string; error?: string }>("/api/settings/test-imap", {
      method: "POST",
      body: JSON.stringify({ host, port, user, password, use_stored: useStored || false }),
    }),
  testAI: (provider: string, model: string, apiKey: string, useStored?: boolean) =>
    apiFetch<{ success: boolean; message?: string; error?: string; provider?: string }>("/api/settings/test-ai", {
      method: "POST",
      body: JSON.stringify({ provider, model, api_key: apiKey, use_stored: useStored || false }),
    }),
  testSMTP: (host: string, port: string, user: string, password: string) =>
    apiFetch<{ success: boolean; message?: string; error?: string }>("/api/settings/test-smtp", {
      method: "POST",
      body: JSON.stringify({ host, port, user, password }),
    }),
  testResend: (apiKey: string, useStored?: boolean) =>
    apiFetch<{ success: boolean; message?: string; error?: string }>("/api/settings/test-resend", {
      method: "POST",
      body: JSON.stringify({ api_key: apiKey, use_stored: useStored || false }),
    }),

  // Analytics
  getSequenceAnalytics: (period: AnalyticsPeriod = "all") =>
    apiFetch<SequenceAnalyticsResponse>(`/api/analytics/sequences?period=${period}`),
  getCostRatios: (period: AnalyticsPeriod = "month") =>
    apiFetch<CostRatiosResponse>(`/api/analytics/cost-ratios?period=${period}`),
  getHotLeads: (params: HotLeadsParams = {}) => {
    const q = new URLSearchParams();
    if (params.period) q.set("period", params.period);
    if (params.status) q.set("status", params.status);
    if (params.channel) q.set("channel", params.channel);
    if (params.limit != null) q.set("limit", String(params.limit));
    const qs = q.toString();
    return apiFetch<HotLeadsResponse>(`/api/analytics/hot-leads${qs ? `?${qs}` : ""}`);
  },
  getInboxAnalytics: (period: AnalyticsPeriod = "month") =>
    apiFetch<InboxFlowResponse>(`/api/analytics/inbox?period=${period}`),
  getQualificationDistribution: (period: AnalyticsPeriod = "all") =>
    apiFetch<QualificationDistributionResponse>(`/api/analytics/qualification-distribution?period=${period}`),
  getSequenceConversion: (period: AnalyticsPeriod = "all") =>
    apiFetch<SequenceConversionResponse>(`/api/analytics/sequence-conversion?period=${period}`),

  // 1C integration settings
  getOnecConfig: () => apiFetch<OnecConfig>("/api/onec/config"),
  updateOnecConfig: (data: OnecConfigUpdate) =>
    apiFetch<OnecConfig>("/api/onec/config", { method: "PUT", body: JSON.stringify(data) }),
  regenerateOnecWebhook: () =>
    apiFetch<{ webhook_secret: string }>("/api/onec/config/regenerate-webhook", { method: "POST" }),
  testOnec: (data: { base_url?: string; auth_type?: string; auth_secret?: string }) =>
    apiFetch<OnecTestResult>("/api/onec/test", { method: "POST", body: JSON.stringify(data) }),
  getOnecMapping: () => apiFetch<OnecMapping>("/api/onec/mapping"),
  updateOnecMapping: (rules: OnecMappingRule[]) =>
    apiFetch<{ saved: boolean }>("/api/onec/mapping", { method: "PUT", body: JSON.stringify({ rules }) }),
  getCostSummary: (from: string, to: string) =>
    apiFetch<CostSummaryResponse>(`/api/audit/cost-summary?from=${from}&to=${to}`),
};

// Types
export interface SourceItem {
  id: string;
  category_id: string;
  name: string;
  sort_order: number;
  created_at: string;
}

export interface SourceCategory {
  id: string;
  name: string;
  sort_order: number;
  sources: SourceItem[];
  created_at: string;
}

export interface SourceStatItem {
  source_id: string;
  source_name: string;
  category_name: string;
  prospect_count: number;
  lead_count: number;
  converted_count: number;
}

export interface EnrichmentProfile {
  title: string;
  description: string;
  emails: string[];
  phones: string[];
  socials: string[];
  // Phase-2 (#186) LLM-classified fields. Optional: legacy/un-enriched
  // profiles omit them, and the API sends "" when unclassified.
  industry?: string;
  companySize?: string;
  // Phase-3 (#188) registry (ЕГРЮЛ) details. Optional/zero when not looked up.
  legal?: LegalDetails;
}

export interface LegalDetails {
  inn?: string;
  ogrn?: string;
  fullName?: string;
  address?: string;
  okved?: string;
  status?: string;
}

export interface Enrichment {
  domain: string;
  status: "pending" | "enriched" | "failed" | "none";
  profile: EnrichmentProfile;
  enrichedAt?: string;
}

export interface Lead {
  id: string;
  user_id: string;
  channel: "telegram" | "email";
  contact_name: string;
  company: string;
  first_message: string;
  status: "new" | "qualified" | "in_conversation" | "followup" | "closed" | "won";
  telegram_chat_id?: number;
  email_address?: string;
  source_id?: string;
  source_name?: string;
  created_at: string;
  updated_at: string;
  /** Set only for archived leads. Present on the archive-view feed and on the
   *  single-lead detail; absent for active leads. Clients use presence to
   *  toggle between the archive and unarchive affordance. */
  archived_at?: string;
  identity?: IdentitySummary;
  /** Count of HITL drafts on this lead awaiting operator decision.
   *  Omitted by the backend when zero; clients default to 0. */
  pending_replies_count?: number;
}

// IdentitySummary surfaces the unified Identity attached to a lead via
// lead_identities. All identifier fields are pre-canonicalized
// server-side (lowercase + trim for email/tg, digits + leading "+"
// for phone). `linked_lead_ids` always includes the current lead when
// the identity is present — clients dedupe when rendering siblings.
export interface IdentitySummary {
  id: string;
  email?: string;
  phone?: string;
  telegram_username?: string;
  linked_lead_ids: string[];
}

export type SuggestionConfidence = "high" | "medium" | "low";

export interface ProspectSuggestion {
  prospect_id: string;
  name: string;
  company: string;
  email: string;
  telegram_username: string;
  source_name: string;
  status: string;
  confidence: SuggestionConfidence;
}

export interface Message {
  id: string;
  lead_id: string;
  direction: "inbound" | "outbound";
  body: string;
  sent_at: string;
}

export interface Qualification {
  id: string;
  lead_id: string;
  identified_need: string;
  estimated_budget: string;
  deadline: string;
  score: number;
  score_reason: string;
  recommended_action: string;
  provider_used: string;
  generated_at: string;
}

export interface Draft {
  id: string;
  lead_id: string;
  body: string;
  created_at: string;
}

export type PendingReplyStatus = "pending" | "approved" | "sent" | "rejected";
export type PendingReplyKind = "booking_link";

export interface PendingReply {
  id: string;
  lead_id: string;
  channel: "telegram" | "email";
  kind: PendingReplyKind;
  body: string;
  status: PendingReplyStatus;
  created_at: string;
  decided_at?: string;
  sent_at?: string;
}

// PendingReplyLeadSnippet is the minimal lead context the operator
// queue needs per row — contact + company + channel + identifiers.
// telegram_chat_id / email_address are omitempty on the wire so a
// telegram lead never carries a null email and vice versa.
export interface PendingReplyLeadSnippet {
  contact_name: string;
  company: string;
  channel: "telegram" | "email";
  telegram_chat_id?: number;
  email_address?: string;
}

// PendingReplyQueueRow is the wire-shape returned by
// GET /api/pending-replies — every pending row across every lead the
// operator owns, joined with the lead snippet so the queue UI avoids
// an N+1 lookup.
export interface PendingReplyQueueRow extends PendingReply {
  lead: PendingReplyLeadSnippet;
}

// PendingReplyBulkDecision matches the BulkDecision enum on the
// backend — the two terminal actions an operator can apply en-masse
// to a slice of pending replies.
export type PendingReplyBulkDecision = "approve" | "reject";

// PendingReplyBulkResult is the per-row outcome surfaced by
// POST /api/pending-replies/bulk. `error` is omitempty server-side
// for success rows, so it's optional on the wire too.
export interface PendingReplyBulkResult {
  id: string;
  ok: boolean;
  error?: string;
}

export interface Reminder {
  id: string;
  lead_id: string;
  message: string;
  snoozed_until?: string;
  dismissed: boolean;
  created_at: string;
}

export interface Prospect {
  id: string;
  user_id: string;
  name: string;
  company: string;
  title: string;
  email: string;
  phone: string;
  whatsapp: string;
  telegram_username: string;
  industry: string;
  company_size: string;
  context: string;
  source: string;
  source_id?: string;
  source_name?: string;
  status: "new" | "in_sequence" | "replied" | "converted" | "opted_out";
  consent_status: "none" | "obtained" | "withdrawn";
  consent_source?: string;
  verify_status: "not_checked" | "valid" | "risky" | "invalid";
  verify_score: number;
  verify_details: Record<string, unknown>;
  verified_at: string | null;
  converted_lead_id: string | null;
  created_at: string;
  updated_at: string;
}

export interface CreateProspectBody {
  name: string;
  company?: string;
  title?: string;
  email: string;
  phone?: string;
  whatsapp?: string;
  telegram_username?: string;
  industry?: string;
  company_size?: string;
  context?: string;
  source_id?: string;
}

export interface EmailVerifyResult {
  email: string;
  is_valid_syntax: boolean;
  has_mx: boolean;
  smtp_valid: boolean;
  smtp_error?: string;
  is_disposable: boolean;
  is_catch_all: boolean;
  is_free_provider: boolean;
  score: number;
  status: "valid" | "risky" | "invalid";
}

export interface VerifyStatus {
  verify_status: "not_checked" | "valid" | "risky" | "invalid";
  verify_score: number;
  verify_details: Record<string, unknown>;
  verified_at: string | null;
}

export interface Sequence {
  id: string;
  user_id: string;
  name: string;
  is_active: boolean;
  created_at: string;
}

export interface SequenceStep {
  id: string;
  sequence_id: string;
  step_order: number;
  delay_days: number;
  prompt_hint: string;
  body: string;
  channel: "email" | "telegram" | "phone_call";
  created_at: string;
}

export interface OutboundMessage {
  id: string;
  prospect_id: string;
  sequence_id: string;
  step_order: number;
  channel: "email" | "telegram" | "phone_call";
  body: string;
  status: "draft" | "approved" | "sent" | "rejected";
  scheduled_at: string;
  sent_at: string | null;
  created_at: string;
}

export interface OutboundStats {
  draft: number;
  approved: number;
  sent: number;
  opened: number;
  replied: number;
  bounced: number;
}

export type AnalyticsPeriod = "week" | "month" | "all";

export interface SequenceAnalyticsRow {
  id: string;
  name: string;
  sent: number;
  delivered: number;
  opened: number;
  replied: number;
  converted: number;
  open_rate: number;
  reply_rate: number;
  conversion_rate: number;
}

export interface SequenceAnalyticsResponse {
  sequences: SequenceAnalyticsRow[];
  period: AnalyticsPeriod;
}

export interface CostRatiosResponse {
  period: { from: string; to: string };
  total_cost_usd: number;
  total_calls: number;
  leads_count: number;
  qualified_leads_count: number;
  converted_count: number;
  drafts_sent_count: number;
  cost_per_lead_usd: number;
  cost_per_qualified_lead_usd: number;
  cost_per_converted_usd: number;
  cost_per_draft_sent_usd: number;
}

export type LeadStatusFilter = "any" | "new" | "qualified" | "in_conversation" | "followup" | "closed";
export type ChannelFilter = "any" | "telegram" | "email";

export interface HotLead {
  id: string;
  contact_name: string;
  channel: string;
  status: string;
  score: number | null;
  score_reason: string;
  last_activity_at: string;
  qualified_at: string | null;
}

export interface HotLeadsResponse {
  leads: HotLead[];
  total_matching: number;
  limit_applied: number;
}

export interface HotLeadsParams {
  period?: AnalyticsPeriod;
  status?: LeadStatusFilter;
  channel?: ChannelFilter;
  limit?: number;
}

export interface QualBucket {
  lo: number;
  hi: number;
  label: string;
  count: number;
}

export interface QualificationDistributionResponse {
  step: number;
  total: number;
  buckets: QualBucket[];
}

export interface SequenceStepConversion {
  sequence_id: string;
  sequence_name: string;
  step_order: number;
  entered: number;
  replied: number;
  advanced: number;
  reply_rate: number;
  advance_rate: number;
}

export interface SequenceConversionResponse {
  steps: SequenceStepConversion[];
}

export interface ScoreBucket {
  range: string;
  count: number;
}

// 1C integration. Secrets (auth_secret, webhook_secret) arrive MASKED from the
// API — never the raw value. Send a new auth_secret only when replacing it; an
// empty or masked value tells the backend to keep the stored one.
export type OnecAuthType = "basic" | "token";

export interface OnecConfig {
  base_url: string;
  auth_type: OnecAuthType;
  auth_secret: string; // masked
  webhook_secret: string; // masked
  is_active: boolean;
}

export interface OnecConfigUpdate {
  base_url?: string;
  auth_type?: OnecAuthType;
  auth_secret?: string;
  is_active?: boolean;
}

export type OnecEventKind = "payment" | "counterparty_created" | "order_status" | "shipment";

export interface OnecMappingRule {
  external_type: string;
  kind: OnecEventKind;
  email_field: string;
  name_field?: string;
  company_field?: string;
}

export interface OnecMapping {
  rules: OnecMappingRule[];
}

export interface OnecTestResult {
  success: boolean;
  error?: string;
}

// InboxFlowResponse is the View 3 (inbox flow) read model. by_channel /
// by_status are open maps keyed by the lead enum members present in the
// period. There is no edited_then_approved field — pending_replies
// stores no original body to diff against, so it's dropped in v1.
export interface InboxFlowResponse {
  period: { from: string; to: string };
  leads: {
    total: number;
    by_channel: Record<string, number>;
    by_status: Record<string, number>;
  };
  qualifications: {
    score_histogram: ScoreBucket[];
    avg_score: number;
  };
  pending_replies: {
    approved: number;
    rejected: number;
    currently_pending: number;
    approve_rate: number;
    p50_time_to_decide_seconds: number;
    p95_time_to_decide_seconds: number;
  };
}

export interface CostBreakdownRow {
  request_type?: string;
  model?: string;
  calls: number;
  usd: number;
  tokens_in: number;
  tokens_out: number;
}

export interface CostSummaryResponse {
  total_usd: number;
  total_calls: number;
  by_request_type: CostBreakdownRow[];
  by_model: CostBreakdownRow[];
  period: { from: string; to: string };
}

export interface UserSettings {
  full_name: string;
  email: string;
  telegram_bot_token: string;
  telegram_bot_active: boolean;
  imap_host: string;
  imap_port: string;
  imap_user: string;
  imap_password: string;
  resend_api_key: string;
  smtp_host: string;
  smtp_port: string;
  smtp_user: string;
  smtp_password: string;
  smtp_active: boolean;
  ai_provider: string;
  ai_model: string;
  ai_api_key: string;
  imap_active: boolean;
  resend_active: boolean;
  ai_active: boolean;
  notify_telegram: boolean;
  notify_email_digest: boolean;
  auto_qualify: boolean;
  auto_draft: boolean;
  auto_send: boolean;
  auto_send_delay_min: number;
  auto_followup: boolean;
  auto_followup_days: number;
  auto_prospect_to_lead: boolean;
  auto_verify_import: boolean;
  ai_style_check_enabled?: boolean;
  aggregated_inbox_view: boolean;
}
