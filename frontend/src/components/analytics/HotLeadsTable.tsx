"use client";

import { useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import type { HotLead } from "@/lib/api";
import { cn } from "@/lib/utils";

interface HotLeadsTableProps {
  leads: HotLead[];
}

type SortKey = "score" | "last_activity_at";
type SortDir = "asc" | "desc";

const STATUS_LABELS: Record<string, string> = {
  new: "Новый",
  qualified: "Квалифицирован",
  in_conversation: "В диалоге",
  followup: "Followup",
  closed: "Закрыт",
};

const CHANNEL_LABELS: Record<string, string> = {
  telegram: "Telegram",
  email: "Email",
};

// scoreBadgeClass colours the score by band so the eye lands on the
// hottest leads first: ≥80 green, 50-79 amber, <50 slate.
function scoreBadgeClass(score: number | null): string {
  if (score == null) return "bg-slate-100 text-slate-400";
  if (score >= 80) return "bg-emerald-100 text-emerald-800";
  if (score >= 50) return "bg-amber-100 text-amber-800";
  return "bg-slate-100 text-slate-600";
}

function formatActivity(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString("ru-RU", { dateStyle: "medium", timeStyle: "short" });
}

export function HotLeadsTable({ leads }: HotLeadsTableProps) {
  const router = useRouter();
  const [sortKey, setSortKey] = useState<SortKey>("score");
  const [sortDir, setSortDir] = useState<SortDir>("desc");

  const sorted = useMemo(() => {
    const copy = [...leads];
    copy.sort((a, b) => {
      let cmp: number;
      if (sortKey === "score") {
        // Null scores always sort last regardless of direction.
        const av = a.score;
        const bv = b.score;
        if (av == null && bv == null) cmp = 0;
        else if (av == null) return 1;
        else if (bv == null) return -1;
        else cmp = av - bv;
      } else {
        cmp = new Date(a.last_activity_at).getTime() - new Date(b.last_activity_at).getTime();
      }
      return sortDir === "asc" ? cmp : -cmp;
    });
    return copy;
  }, [leads, sortKey, sortDir]);

  function handleSort(key: SortKey) {
    if (key === sortKey) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("desc");
    }
  }

  if (leads.length === 0) {
    return (
      <div className="rounded-lg border border-slate-200 bg-white p-8 text-center text-sm text-slate-500">
        Нет лидов под выбранные фильтры.
      </div>
    );
  }

  const sortMark = (key: SortKey) => (sortKey === key ? (sortDir === "asc" ? "↑" : "↓") : null);

  return (
    <div className="overflow-x-auto rounded-lg border border-slate-200 bg-white">
      <table className="min-w-full divide-y divide-slate-200 text-sm">
        <thead className="bg-slate-50">
          <tr>
            <th scope="col" className="px-4 py-2 text-left font-medium text-slate-700">Контакт</th>
            <th scope="col" className="px-4 py-2 text-left font-medium text-slate-700">Канал</th>
            <th scope="col" className="px-4 py-2 text-left font-medium text-slate-700">Статус</th>
            <th scope="col" className="px-4 py-2 text-right font-medium text-slate-700">
              <button type="button" onClick={() => handleSort("score")} className="inline-flex items-center gap-1 hover:text-slate-900">
                Скор {sortMark("score") && <span aria-hidden="true">{sortMark("score")}</span>}
              </button>
            </th>
            <th scope="col" className="px-4 py-2 text-right font-medium text-slate-700">
              <button type="button" onClick={() => handleSort("last_activity_at")} className="inline-flex items-center gap-1 hover:text-slate-900">
                Активность {sortMark("last_activity_at") && <span aria-hidden="true">{sortMark("last_activity_at")}</span>}
              </button>
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100">
          {sorted.map((lead) => (
            <tr
              key={lead.id}
              onClick={() => router.push(`/inbox/${lead.id}`)}
              className="cursor-pointer hover:bg-slate-50"
            >
              <td className="px-4 py-2 font-medium text-slate-900">{lead.contact_name || "—"}</td>
              <td className="px-4 py-2 text-slate-600">{CHANNEL_LABELS[lead.channel] ?? lead.channel}</td>
              <td className="px-4 py-2 text-slate-600">{STATUS_LABELS[lead.status] ?? lead.status}</td>
              <td className="px-4 py-2 text-right">
                <span
                  title={lead.score_reason || undefined}
                  className={cn("inline-block rounded px-2 py-0.5 text-xs font-semibold tabular-nums", scoreBadgeClass(lead.score))}
                >
                  {lead.score ?? "—"}
                </span>
              </td>
              <td className="px-4 py-2 text-right text-slate-500 tabular-nums">{formatActivity(lead.last_activity_at)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
