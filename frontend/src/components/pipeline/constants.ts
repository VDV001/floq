import type { Lead } from "@/lib/api";

export { getTimeAgo } from "@/lib/format";

export type Channel = "all" | "email" | "telegram";

export interface PipelineLead {
  id: string;
  name: string;
  company: string;
  channel: "email" | "telegram";
  preview?: string;
  timeAgo: string;
}

export interface PipelineColumn {
  key: string;
  title: string;
  count: number;
  dotColor: string;
  badgeStyle: string;
  leads: PipelineLead[];
}

export const STATUS_CONFIG: Record<Lead["status"], { key: string; title: string; dotColor: string; badgeStyle: string }> = {
  new: { key: "new", title: "Новый", dotColor: "#004ac6", badgeStyle: "bg-blue-50 text-blue-600" },
  qualified: { key: "qualified", title: "Квалифицирован", dotColor: "#3e3fcc", badgeStyle: "bg-purple-50 text-purple-600" },
  in_conversation: { key: "in_conversation", title: "В диалоге", dotColor: "#f59e0b", badgeStyle: "border border-amber-300 bg-amber-50 text-amber-700" },
  followup: { key: "followup", title: "Фоллоуап", dotColor: "#f97316", badgeStyle: "border border-orange-300 bg-orange-50 text-orange-700" },
  closed: { key: "closed", title: "Закрыт", dotColor: "#10b981", badgeStyle: "border border-green-300 bg-green-50 text-green-700" },
  won: { key: "won", title: "Выигран", dotColor: "#059669", badgeStyle: "border border-emerald-400 bg-emerald-50 text-emerald-800" },
};

export const COLUMN_ORDER: Lead["status"][] = ["new", "qualified", "in_conversation", "followup", "won", "closed"];

export const CHANNEL_FILTERS: { label: string; value: Channel }[] = [
  { label: "Все каналы", value: "all" },
  { label: "Email", value: "email" },
  { label: "Telegram", value: "telegram" },
];
