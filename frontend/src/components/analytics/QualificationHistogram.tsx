"use client";

import type { ScoreBucket } from "@/lib/api";

interface QualificationHistogramProps {
  histogram: ScoreBucket[];
  avgScore: number;
}

// QualificationHistogram draws the qualification-score distribution as a
// set of horizontal bars (one per 0-100 band) plus the mean score. Bars
// are scaled against the tallest band so the shape reads at a glance
// without a charting dependency. An all-zero histogram shows an empty
// state rather than five flat bars.
export function QualificationHistogram({ histogram, avgScore }: QualificationHistogramProps) {
  const max = histogram.reduce((m, b) => Math.max(m, b.count), 0);

  if (max === 0) {
    return (
      <div className="rounded-lg border border-slate-200 bg-white p-6">
        <h2 className="text-sm font-semibold text-slate-700">Распределение скоров</h2>
        <p className="mt-4 text-center text-sm text-slate-500">Нет квалификаций за период.</p>
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-slate-200 bg-white p-6">
      <div className="flex items-baseline justify-between">
        <h2 className="text-sm font-semibold text-slate-700">Распределение скоров</h2>
        <span className="text-sm text-slate-500">
          Средний: <span className="font-semibold text-slate-900 tabular-nums">{avgScore.toFixed(1)}</span>
        </span>
      </div>
      <div className="mt-4 space-y-2">
        {histogram.map((b) => {
          const pct = Math.round((b.count / max) * 100);
          return (
            <div key={b.range} className="flex items-center gap-3 text-sm">
              <span className="w-16 shrink-0 text-right tabular-nums text-slate-500">{b.range}</span>
              <div className="relative h-5 flex-1 overflow-hidden rounded bg-slate-100">
                <div
                  data-testid={`bar-fill-${b.range}`}
                  className="h-full rounded bg-indigo-500"
                  style={{ width: `${pct}%` }}
                />
              </div>
              <span data-testid={`bar-count-${b.range}`} className="w-8 shrink-0 tabular-nums text-slate-900">
                {b.count}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
