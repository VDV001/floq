"use client";

import { cn } from "@/lib/utils";
import type { AnalyticsPeriod } from "@/lib/api";

const OPTIONS: { value: AnalyticsPeriod; label: string }[] = [
  { value: "week", label: "Неделя" },
  { value: "month", label: "Месяц" },
  { value: "all", label: "Всё время" },
];

interface PeriodSelectorProps {
  value: AnalyticsPeriod;
  onChange: (next: AnalyticsPeriod) => void;
}

export function PeriodSelector({ value, onChange }: PeriodSelectorProps) {
  return (
    <div role="radiogroup" aria-label="Период" className="inline-flex rounded-md border border-slate-200 bg-white p-1">
      {OPTIONS.map((opt) => (
        <button
          key={opt.value}
          role="radio"
          aria-checked={value === opt.value}
          onClick={() => onChange(opt.value)}
          className={cn(
            "px-3 py-1.5 text-sm font-medium rounded transition-colors",
            value === opt.value
              ? "bg-slate-900 text-white"
              : "text-slate-600 hover:bg-slate-100",
          )}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
}
