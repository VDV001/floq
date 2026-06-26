"use client";

import Link from "next/link";
import { Link2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { LeadAvatar } from "@/components/leads/LeadAvatar";
import { STATUS_STYLES, type LeadStatus } from "@/components/leads/constants";

export interface LeadCardProps {
  id: string;
  company: string;
  contact: string;
  channel: "email" | "telegram";
  preview: string;
  timeAgo: string;
  status: LeadStatus;
  sourceName?: string;
  /** Count of HITL drafts on this lead awaiting operator decision.
   *  Renders an amber badge when > 0; absent or 0 means no badge. */
  pendingRepliesCount?: number;
  /** Count of cross-channel prospect-dedup suggestions for this lead.
   *  Renders a beige Link2 badge when > 0. */
  suggestionCount?: number;
}

export function LeadCard({
  id,
  company,
  contact,
  channel,
  preview,
  timeAgo,
  status,
  sourceName,
  pendingRepliesCount,
  suggestionCount,
}: LeadCardProps) {
  return (
    <Link
      href={`/inbox/${id}`}
      className="group relative flex cursor-pointer rounded-xl border border-transparent bg-white p-5 transition-all hover:border-[#c3c6d7]/10 hover:bg-[#dce9ff]/40"
    >
      <div className="flex items-start gap-4 flex-1 min-w-0">
        <LeadAvatar channel={channel} />
        <div className="min-w-0 flex-1">
          <h4 className="font-bold leading-none text-[#0d1c2e]">{company}</h4>
          <p className="mt-1 text-xs font-medium text-[#737686]">
            {channel === "email" ? "по email" : "через Telegram"} · {contact}
          </p>
          {/* Wrapper is unconditional to preserve mt-2 spacing even when
              sourceName is absent — matches the pre-refactor inbox JSX. */}
          <div className="mt-2 flex items-center gap-2">
            {sourceName && (
              <span className="rounded-full bg-[#eff4ff] px-2 py-0.5 text-[10px] font-semibold text-[#004ac6]">
                {sourceName}
              </span>
            )}
          </div>
          <p className="mt-1 line-clamp-2 text-sm leading-relaxed text-[#434655]">
            {preview}
          </p>
        </div>
      </div>
      <div className="ml-4 flex shrink-0 flex-col items-end gap-2">
        <span className="text-[10px] font-bold uppercase tracking-wider text-[#737686]">
          {timeAgo}
        </span>
        <span
          className={cn(
            "whitespace-nowrap rounded-full px-3 py-1 text-[10px] font-bold",
            STATUS_STYLES[status]
          )}
        >
          {status}
        </span>
        {suggestionCount !== undefined && suggestionCount > 0 && (
          <span
            aria-label={`${suggestionCount} возможных совпадений с проспектом`}
            className="inline-flex items-center gap-1 rounded-full bg-[#fff3cd] px-2 py-0.5 text-[10px] font-semibold text-[#8a5a00]"
            title={`${suggestionCount} возможных совпадений с проспектом`}
          >
            <Link2 className="size-3" />
            {suggestionCount}
          </span>
        )}
        {pendingRepliesCount !== undefined && pendingRepliesCount > 0 && (
          <span
            aria-label={`${pendingRepliesCount} ожидают подтверждения`}
            className="inline-flex items-center gap-1 rounded-full bg-[#f5b73c] px-2 py-0.5 text-[10px] font-bold text-[#0d1c2e]"
            title={`${pendingRepliesCount} ожидают подтверждения`}
          >
            ⏵ {pendingRepliesCount}
          </span>
        )}
      </div>
    </Link>
  );
}
