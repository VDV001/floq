"use client";

import { useMemo, useState } from "react";
import type { SequenceAnalyticsRow } from "@/lib/api";

type SortKey = "name" | "sent" | "delivered" | "opened" | "replied" | "converted" | "open_rate" | "reply_rate" | "conversion_rate";
type SortDir = "asc" | "desc";

interface SequenceStatsTableProps {
  rows: SequenceAnalyticsRow[];
}

const COLUMNS: { key: SortKey; label: string; numeric: boolean }[] = [
  { key: "name", label: "Sequence", numeric: false },
  { key: "sent", label: "Sent", numeric: true },
  { key: "delivered", label: "Delivered", numeric: true },
  { key: "opened", label: "Opened", numeric: true },
  { key: "replied", label: "Replied", numeric: true },
  { key: "converted", label: "Converted", numeric: true },
  { key: "open_rate", label: "Open %", numeric: true },
  { key: "reply_rate", label: "Reply %", numeric: true },
  { key: "conversion_rate", label: "Conv. %", numeric: true },
];

function formatPercent(v: number): string {
  return `${(v * 100).toFixed(1)}%`;
}

export function SequenceStatsTable({ rows }: SequenceStatsTableProps) {
  const [sortKey, setSortKey] = useState<SortKey>("sent");
  const [sortDir, setSortDir] = useState<SortDir>("desc");

  const sorted = useMemo(() => {
    const copy = [...rows];
    copy.sort((a, b) => {
      const av = a[sortKey];
      const bv = b[sortKey];
      const cmp =
        typeof av === "number" && typeof bv === "number"
          ? av - bv
          : String(av).localeCompare(String(bv));
      return sortDir === "asc" ? cmp : -cmp;
    });
    return copy;
  }, [rows, sortKey, sortDir]);

  function handleSort(key: SortKey) {
    if (key === sortKey) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortKey(key);
      setSortDir("desc");
    }
  }

  if (rows.length === 0) {
    return (
      <div className="rounded-lg border border-slate-200 bg-white p-8 text-center text-sm text-slate-500">
        Нет sequence&apos;ов с активностью в выбранном периоде.
      </div>
    );
  }

  return (
    <div className="overflow-x-auto rounded-lg border border-slate-200 bg-white">
      <table className="min-w-full divide-y divide-slate-200 text-sm">
        <thead className="bg-slate-50">
          <tr>
            {COLUMNS.map((col) => (
              <th
                key={col.key}
                scope="col"
                className={`px-4 py-2 font-medium text-slate-700 ${col.numeric ? "text-right" : "text-left"}`}
              >
                <button
                  type="button"
                  onClick={() => handleSort(col.key)}
                  className="inline-flex items-center gap-1 hover:text-slate-900"
                >
                  {col.label}
                  {sortKey === col.key && <span aria-hidden="true">{sortDir === "asc" ? "↑" : "↓"}</span>}
                </button>
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100">
          {sorted.map((row) => (
            <tr key={row.id}>
              <td className="px-4 py-2 font-medium text-slate-900">{row.name}</td>
              <td className="px-4 py-2 text-right tabular-nums">{row.sent}</td>
              <td className="px-4 py-2 text-right tabular-nums">{row.delivered}</td>
              <td className="px-4 py-2 text-right tabular-nums">{row.opened}</td>
              <td className="px-4 py-2 text-right tabular-nums">{row.replied}</td>
              <td className="px-4 py-2 text-right tabular-nums">{row.converted}</td>
              <td className="px-4 py-2 text-right tabular-nums">{formatPercent(row.open_rate)}</td>
              <td className="px-4 py-2 text-right tabular-nums">{formatPercent(row.reply_rate)}</td>
              <td className="px-4 py-2 text-right tabular-nums">{formatPercent(row.conversion_rate)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
