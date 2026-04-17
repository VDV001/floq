import type { OutboundMessage } from "@/lib/api";

const AVATAR_BGS = ["bg-[#d5e0f8]", "bg-[#e1e0ff]", "bg-[#d8e3fb]", "bg-[#dbe1ff]", "bg-[#d5f0e8]"];

export interface UIMessage {
  id: string;
  name: string;
  initials: string;
  role: string;
  avatarBg: string;
  step: string;
  sequence: string;
  body: string;
  scheduledAt: string;
  channel: "email" | "telegram" | "phone_call";
  status: string;
}

function formatScheduledAt(iso: string): string {
  const d = new Date(iso);
  const now = new Date();
  const isToday = d.getDate() === now.getDate() && d.getMonth() === now.getMonth() && d.getFullYear() === now.getFullYear();
  const time = d.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit" });
  return isToday ? `сегодня, ${time}` : d.toLocaleDateString("ru-RU") + ", " + time;
}

export function mapOutboundToUI(msg: OutboundMessage, idx: number): UIMessage {
  const prospectLabel = `Проспект ${msg.prospect_id.slice(0, 6)}`;
  const initials = prospectLabel.split(" ").map((w) => w[0]).join("").slice(0, 2).toUpperCase();
  return {
    id: msg.id, name: prospectLabel, initials, role: "",
    avatarBg: AVATAR_BGS[idx % AVATAR_BGS.length],
    step: `Шаг ${msg.step_order}`, sequence: msg.sequence_id.slice(0, 8),
    body: msg.body, scheduledAt: formatScheduledAt(msg.scheduled_at),
    channel: msg.channel, status: msg.status,
  };
}

export function formatTime(d: Date): string {
  return d.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}
