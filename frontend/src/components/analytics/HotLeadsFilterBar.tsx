"use client";

import type { ChannelFilter, LeadStatusFilter } from "@/lib/api";

const STATUS_OPTIONS: { value: LeadStatusFilter; label: string }[] = [
  { value: "any", label: "Все статусы" },
  { value: "new", label: "Новые" },
  { value: "qualified", label: "Квалифицированы" },
  { value: "in_conversation", label: "В диалоге" },
  { value: "followup", label: "Followup" },
  { value: "closed", label: "Закрытые" },
];

const CHANNEL_OPTIONS: { value: ChannelFilter; label: string }[] = [
  { value: "any", label: "Все каналы" },
  { value: "telegram", label: "Telegram" },
  { value: "email", label: "Email" },
];

interface HotLeadsFilterBarProps {
  status: LeadStatusFilter;
  channel: ChannelFilter;
  onStatusChange: (next: LeadStatusFilter) => void;
  onChannelChange: (next: ChannelFilter) => void;
}

const selectClass =
  "rounded-md border border-slate-200 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50 focus:outline-none focus:ring-2 focus:ring-slate-300";

export function HotLeadsFilterBar({ status, channel, onStatusChange, onChannelChange }: HotLeadsFilterBarProps) {
  return (
    <div className="flex items-center gap-3">
      <label className="sr-only" htmlFor="hot-leads-status">
        Статус
      </label>
      <select
        id="hot-leads-status"
        className={selectClass}
        value={status}
        onChange={(e) => onStatusChange(e.target.value as LeadStatusFilter)}
      >
        {STATUS_OPTIONS.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>

      <label className="sr-only" htmlFor="hot-leads-channel">
        Канал
      </label>
      <select
        id="hot-leads-channel"
        className={selectClass}
        value={channel}
        onChange={(e) => onChannelChange(e.target.value as ChannelFilter)}
      >
        {CHANNEL_OPTIONS.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </div>
  );
}
