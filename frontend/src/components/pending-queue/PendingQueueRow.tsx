"use client";

import Link from "next/link";
import { ShieldCheck, X, Mail, Send } from "lucide-react";
import type { PendingReplyQueueRow } from "@/lib/api";

interface Props {
  row: PendingReplyQueueRow;
  onApprove: (id: string) => void;
  onReject: (id: string) => void;
  busy?: boolean;
  selected?: boolean;
  onToggleSelect?: (id: string) => void;
}

const KIND_LABELS: Record<PendingReplyQueueRow["kind"], string> = {
  booking_link: "Ссылка на встречу",
};

const CHANNEL_LABELS: Record<PendingReplyQueueRow["channel"], string> = {
  telegram: "Telegram",
  email: "Email",
};

export function PendingQueueRow({
  row,
  onApprove,
  onReject,
  busy = false,
  selected,
  onToggleSelect,
}: Props) {
  const isEmail = row.channel === "email";
  const selectable = onToggleSelect !== undefined;
  return (
    <li className="rounded-xl border border-[#f5b73c]/30 bg-white p-5 transition-shadow hover:shadow-sm">
      <header className="mb-3 flex items-start justify-between gap-3">
        <div className="flex items-center gap-3">
          {selectable && (
            <input
              type="checkbox"
              checked={!!selected}
              onChange={() => onToggleSelect!(row.id)}
              aria-label={`Выбрать драфт для ${row.lead.contact_name || "лида"}`}
              className="size-4 cursor-pointer accent-[#004ac6]"
            />
          )}
          <div
            className={`flex size-10 items-center justify-center rounded-full ${
              isEmail ? "bg-[#dbe1ff]" : "bg-[#d5e0f8]"
            }`}
            aria-hidden
          >
            {isEmail ? (
              <Mail className="size-5 text-[#0d1c2e]" />
            ) : (
              <Send className="size-5 text-[#0d1c2e]" />
            )}
          </div>
          <div className="min-w-0">
            <Link
              href={`/inbox/${row.lead_id}`}
              className="block truncate text-sm font-bold text-[#0d1c2e] hover:text-[#004ac6] hover:underline"
            >
              {row.lead.contact_name || "Без имени"}
              {row.lead.company ? ` · ${row.lead.company}` : ""}
            </Link>
            <p className="mt-0.5 text-[11px] font-medium text-[#737686]">
              {KIND_LABELS[row.kind] ?? row.kind} · {CHANNEL_LABELS[row.channel] ?? row.channel}
            </p>
          </div>
        </div>
      </header>

      <p className="mb-4 whitespace-pre-wrap text-sm text-[#0d1c2e]">{row.body}</p>

      <div className="flex justify-end gap-2">
        <button
          type="button"
          onClick={() => onReject(row.id)}
          disabled={busy}
          className="inline-flex items-center gap-1.5 rounded-lg border border-[#c3c6d7]/50 bg-white px-3 py-1.5 text-xs font-semibold text-[#434655] transition-colors hover:bg-[#f7f9fd] disabled:opacity-50"
        >
          <X className="size-3.5" />
          Отклонить
        </button>
        <button
          type="button"
          onClick={() => onApprove(row.id)}
          disabled={busy}
          className="inline-flex items-center gap-1.5 rounded-lg bg-[#0d7a2c] px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-[#0a6324] disabled:opacity-50"
        >
          <ShieldCheck className="size-3.5" />
          Одобрить и отправить
        </button>
      </div>
    </li>
  );
}
