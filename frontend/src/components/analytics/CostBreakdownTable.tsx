"use client";

import { useMemo, useState } from "react";
import type { CostBreakdownRow } from "@/lib/api";

type SortKey = "label" | "calls" | "usd" | "tokens_in" | "tokens_out";
type SortDir = "asc" | "desc";

interface CostBreakdownTableProps {
  title: string;
  labelHeader: string;
  rows: CostBreakdownRow[];
  // labelKey picks which CostBreakdownRow field (request_type or model)
  // backs the first column. Keeps the component reusable for both
  // breakdowns without an `any` cast.
  labelKey: "request_type" | "model";
}

function formatUSD(v: number): string {
  if (v < 0.01) return `$${v.toFixed(4)}`;
  return `$${v.toFixed(2)}`;
}

export function CostBreakdownTable({ title, labelHeader, rows, labelKey }: CostBreakdownTableProps) {
  const [sortKey, setSortKey] = useState<SortKey>("usd");
  const [sortDir, setSortDir] = useState<SortDir>("desc");

  const sorted = useMemo(() => {
    const copy = [...rows];
    copy.sort((a, b) => {
      let cmp: number;
      if (sortKey === "label") {
        const la = String(a[labelKey] ?? "");
        const lb = String(b[labelKey] ?? "");
        cmp = la.localeCompare(lb);
      } else {
        cmp = (a[sortKey] as number) - (b[sortKey] as number);
      }
      return sortDir === "asc" ? cmp : -cmp;
    });
    return copy;
  }, [rows, sortKey, sortDir, labelKey]);

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
      <div className="rounded-lg border border-slate-200 bg-white p-6 text-center text-sm text-slate-500">
        {title}: нет данных за период.
      </div>
    );
  }

  const columns: { key: SortKey; label: string; numeric: boolean }[] = [
    { key: "label", label: labelHeader, numeric: false },
    { key: "calls", label: "Вызовов", numeric: true },
    { key: "usd", label: "USD", numeric: true },
    { key: "tokens_in", label: "Tokens in", numeric: true },
    { key: "tokens_out", label: "Tokens out", numeric: true },
  ];

  return (
    <div className="rounded-lg border border-slate-200 bg-white">
      <h2 className="px-4 py-2 text-sm font-semibold text-slate-700 border-b border-slate-100">{title}</h2>
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-slate-200 text-sm">
          <thead className="bg-slate-50">
            <tr>
              {columns.map((col) => (
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
            {sorted.map((row, idx) => (
              <tr key={`${row[labelKey] ?? "row"}-${idx}`}>
                <td className="px-4 py-2 font-medium text-slate-900">{row[labelKey] ?? "—"}</td>
                <td className="px-4 py-2 text-right tabular-nums">{row.calls}</td>
                <td className="px-4 py-2 text-right tabular-nums">{formatUSD(row.usd)}</td>
                <td className="px-4 py-2 text-right tabular-nums">{row.tokens_in}</td>
                <td className="px-4 py-2 text-right tabular-nums">{row.tokens_out}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
