"use client";

import Link from "next/link";
import { Mail, Send } from "lucide-react";
import { cn } from "@/lib/utils";

export interface LeadCardProps {
  id: string;
  company: string;
  contact: string;
  channel: "email" | "telegram";
  preview: string;
  timeAgo: string;
  status: "Новый" | "Квалифицирован" | "Нужен фоллоуап";
  /** Count of HITL drafts on this lead awaiting operator decision.
   *  Renders a small badge when > 0; absent or 0 means no badge. */
  pendingRepliesCount?: number;
}

const STATUS_STYLES: Record<LeadCardProps["status"], string> = {
  "Новый": "bg-[#3b6ef6]/10 text-[#3b6ef6]",
  "Квалифицирован": "border border-[#3b6ef6] text-[#3b6ef6] bg-transparent",
  "Нужен фоллоуап": "bg-[#f59e0b]/10 text-[#f59e0b]",
};

export function LeadCard({
  id,
  company,
  contact,
  channel,
  preview,
  timeAgo,
  status,
  pendingRepliesCount,
}: LeadCardProps) {
  return (
    <Link
      href={`/inbox/${id}`}
      className="group flex items-start gap-4 rounded-xl bg-white p-4 transition-shadow hover:shadow-md"
    >
      {/* Channel Icon */}
      <div
        className={cn(
          "flex size-12 shrink-0 items-center justify-center rounded-xl",
          channel === "email" ? "bg-[#3b6ef6]/10" : "bg-[#3b6ef6]/10"
        )}
      >
        {channel === "email" ? (
          <Mail className="size-5 text-[#3b6ef6]" />
        ) : (
          <Send className="size-5 text-[#3b6ef6]" />
        )}
      </div>

      {/* Content */}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-semibold text-[#0d1c2e]">{company}</h3>
        </div>
        <p className="text-xs text-[#6b7280]">
          {channel === "email" ? "по email" : "через Telegram"} &middot; {contact}
        </p>
        <p className="mt-1 truncate text-sm text-[#6b7280]">{preview}</p>
      </div>

      {/* Right: time + badge */}
      <div className="flex shrink-0 flex-col items-end gap-2">
        <span className="text-[10px] font-medium tracking-wide text-[#6b7280] uppercase">
          {timeAgo}
        </span>
        <span
          className={cn(
            "rounded-full px-2.5 py-0.5 text-[10px] font-semibold whitespace-nowrap",
            STATUS_STYLES[status]
          )}
        >
          {status}
        </span>
        {pendingRepliesCount !== undefined && pendingRepliesCount > 0 && (
          <span
            aria-label={`${pendingRepliesCount} ожидают подтверждения`}
            className="inline-flex items-center gap-1 rounded-full bg-[#f5b73c] px-2 py-0.5 text-[10px] font-bold text-[#0d1c2e]"
          >
            ⏵ {pendingRepliesCount}
          </span>
        )}
      </div>
    </Link>
  );
}
