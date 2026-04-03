const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

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
          if (retryRes.ok) return retryRes.json();
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
    throw new Error(`API error: ${res.status} ${res.statusText}`);
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

  // Leads
  getLeads: () => apiFetch<Lead[]>("/api/leads"),
  getLead: (id: string) => apiFetch<Lead>(`/api/leads/${id}`),
  updateLeadStatus: (id: string, status: string) =>
    apiFetch(`/api/leads/${id}/status`, {
      method: "PATCH",
      body: JSON.stringify({ status }),
    }),

  // Messages
  getMessages: (leadId: string) =>
    apiFetch<Message[]>(`/api/leads/${leadId}/messages`),
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

  // Reminders
  getReminders: () => apiFetch<Reminder[]>("/api/reminders"),
  snoozeReminder: (id: string) =>
    apiFetch(`/api/reminders/${id}/snooze`, { method: "POST" }),
  dismissReminder: (id: string) =>
    apiFetch(`/api/reminders/${id}/dismiss`, { method: "POST" }),

  // Prospects
  getProspects: () => apiFetch<Prospect[]>("/api/prospects"),
  createProspect: (data: CreateProspectBody) =>
    apiFetch<Prospect>("/api/prospects", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  deleteProspect: (id: string) =>
    apiFetch(`/api/prospects/${id}`, { method: "DELETE" }),

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

  // Sequences
  getSequences: () => apiFetch<Sequence[]>("/api/sequences"),
  getSequence: (id: string) => apiFetch<{ sequence: Sequence; steps: SequenceStep[] }>(`/api/sequences/${id}`),
  createSequence: (name: string) =>
    apiFetch<Sequence>("/api/sequences", { method: "POST", body: JSON.stringify({ name }) }),
  deleteSequence: (id: string) =>
    apiFetch(`/api/sequences/${id}`, { method: "DELETE" }),
  addStep: (seqId: string, data: { step_order: number; delay_days: number; channel: string; prompt_hint: string }) =>
    apiFetch<SequenceStep>(`/api/sequences/${seqId}/steps`, { method: "POST", body: JSON.stringify(data) }),
  launchSequence: (seqId: string, prospectIds: string[]) =>
    apiFetch(`/api/sequences/${seqId}/launch`, { method: "POST", body: JSON.stringify({ prospect_ids: prospectIds }) }),
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
  getOutboundStats: () => apiFetch<OutboundStats>("/api/outbound/stats"),

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
};

// Types
export interface Lead {
  id: string;
  user_id: string;
  channel: "telegram" | "email";
  contact_name: string;
  company: string;
  first_message: string;
  status: "new" | "qualified" | "in_conversation" | "followup" | "closed";
  telegram_chat_id?: number;
  email_address?: string;
  created_at: string;
  updated_at: string;
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
  telegram_username: string;
  industry: string;
  company_size: string;
  context: string;
  source: "manual" | "csv";
  status: "new" | "in_sequence" | "replied" | "converted" | "opted_out";
  verify_status: "not_checked" | "valid" | "risky" | "invalid";
  verify_score: number;
  verify_details: Record<string, any>;
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
  telegram_username?: string;
  industry?: string;
  company_size?: string;
  context?: string;
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
  verify_details: Record<string, any>;
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
  ai_provider: string;
  ai_model: string;
  ai_api_key: string;
  notify_telegram: boolean;
  notify_email_digest: boolean;
}
