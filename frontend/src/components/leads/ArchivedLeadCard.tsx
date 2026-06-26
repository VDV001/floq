"use client";

import Link from "next/link";
import { ArchiveRestore } from "lucide-react";
import { cn } from "@/lib/utils";
import { LeadAvatar } from "@/components/leads/LeadAvatar";
import { STATUS_STYLES, type LeadStatus } from "@/components/leads/constants";

export interface ArchivedLeadCardProps {
  id: string;
  company: string;
  contact: string;
  channel: "email" | "telegram";
  status: LeadStatus;
  sourceName?: string;
  /** Ready-to-render label for when the lead was archived (e.g. "5 мин назад"
   *  or "Только что") — already phrased, the card does not append "назад". */
  archivedLabel: string;
  /** True while this card's unarchive request is in flight. */
  unarchiving: boolean;
  onUnarchive: (id: string) => void;
}

// ArchivedLeadCard renders one row of the archive view. Unlike LeadCard the
// whole row is NOT a single anchor — the body links to the lead detail while
// the unarchive control is a sibling button, so we never nest a button inside
// an <a> (invalid markup + swallowed clicks).
export function ArchivedLeadCard({
  id,
  company,
  contact,
  channel,
  status,
  sourceName,
  archivedLabel,
  unarchiving,
  onUnarchive,
}: ArchivedLeadCardProps) {
  return (
    <div className="group relative flex items-center gap-4 rounded-xl border border-transparent bg-white p-5 transition-all hover:border-[#c3c6d7]/10 hover:bg-[#dce9ff]/40">
      <Link href={`/inbox/${id}`} className="flex min-w-0 flex-1 items-start gap-4">
        <LeadAvatar channel={channel} />
        <div className="min-w-0 flex-1">
          <h4 className="font-bold leading-none text-[#0d1c2e]">{company}</h4>
          <p className="mt-1 text-xs font-medium text-[#737686]">
            {channel === "email" ? "по email" : "через Telegram"} · {contact}
          </p>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            {sourceName && (
              <span className="rounded-full bg-[#eff4ff] px-2 py-0.5 text-[10px] font-semibold text-[#004ac6]">
                {sourceName}
              </span>
            )}
            <span className="rounded-full bg-[#eef0f4] px-2 py-0.5 text-[10px] font-semibold text-[#737686]">
              В архиве · {archivedLabel}
            </span>
          </div>
        </div>
      </Link>
      <div className="flex shrink-0 flex-col items-end gap-2">
        <span
          className={cn(
            "whitespace-nowrap rounded-full px-3 py-1 text-[10px] font-bold",
            STATUS_STYLES[status]
          )}
        >
          {status}
        </span>
        <button
          onClick={() => onUnarchive(id)}
          disabled={unarchiving}
          className="inline-flex items-center gap-1.5 rounded-lg border border-[#c3c6d7]/30 bg-white px-3 py-1.5 text-xs font-semibold text-[#0d1c2e] transition-colors hover:bg-[#eff4ff] disabled:cursor-not-allowed disabled:opacity-60"
        >
          <ArchiveRestore className="size-3.5" />
          {unarchiving ? "Возвращаем…" : "Разархивировать"}
        </button>
      </div>
    </div>
  );
}
