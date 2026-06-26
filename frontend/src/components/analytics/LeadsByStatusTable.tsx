"use client";

import { useRouter } from "next/navigation";

interface LeadsByStatusTableProps {
  byStatus: Record<string, number>;
  total: number;
}

// STATUSES is the lead_status enum in funnel order, with Russian labels.
// Rendered in this fixed order so the table reads as a funnel and a
// status with zero leads still appears (as a 0 row) rather than
// reshuffling between periods.
const STATUSES: { key: string; label: string }[] = [
  { key: "new", label: "Новые" },
  { key: "qualified", label: "Квалифицированные" },
  { key: "in_conversation", label: "В диалоге" },
  { key: "followup", label: "Follow-up" },
  { key: "closed", label: "Закрытые" },
];

// LeadsByStatusTable lists lead volume per status with share-of-total.
// A row click drills through to the inbox lead list. Pre-selecting the
// clicked status there is a follow-up — the inbox filter is local state
// today, not a URL param — so the click lands on the unfiltered inbox.
export function LeadsByStatusTable({ byStatus, total }: LeadsByStatusTableProps) {
  const router = useRouter();

  if (total === 0) {
    return (
      <div className="rounded-lg border border-slate-200 bg-white p-6">
        <h2 className="text-sm font-semibold text-slate-700">Лиды по статусам</h2>
        <p className="mt-4 text-center text-sm text-slate-500">Нет лидов за период.</p>
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-slate-200 bg-white">
      <h2 className="px-4 py-2 text-sm font-semibold text-slate-700 border-b border-slate-100">
        Лиды по статусам
      </h2>
      <table className="min-w-full divide-y divide-slate-200 text-sm">
        <thead className="bg-slate-50">
          <tr>
            <th scope="col" className="px-4 py-2 text-left font-medium text-slate-700">Статус</th>
            <th scope="col" className="px-4 py-2 text-right font-medium text-slate-700">Лидов</th>
            <th scope="col" className="px-4 py-2 text-right font-medium text-slate-700">Доля</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100">
          {STATUSES.map((s) => {
            const count = byStatus[s.key] ?? 0;
            const pct = total > 0 ? Math.round((count / total) * 100) : 0;
            return (
              <tr
                key={s.key}
                onClick={() => router.push("/inbox")}
                className="cursor-pointer hover:bg-slate-50"
              >
                <td className="px-4 py-2 font-medium text-slate-900">{s.label}</td>
                <td className="px-4 py-2 text-right tabular-nums">{count}</td>
                <td className="px-4 py-2 text-right tabular-nums text-slate-500">{pct}%</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
