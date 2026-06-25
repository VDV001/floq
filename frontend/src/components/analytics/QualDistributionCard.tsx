"use client";

import type { QualificationDistributionResponse } from "@/lib/api";

interface QualDistributionCardProps {
  data: QualificationDistributionResponse;
}

// QualDistributionCard renders the matview-backed qualification-score
// distribution (funnel view) as a horizontal bar per bucket. Distinct from
// QualificationHistogram, which renders the inbox view's live score
// histogram. Bar width is proportional to the largest bucket.
export function QualDistributionCard({ data }: QualDistributionCardProps) {
  const maxCount = data.buckets.reduce((m, b) => Math.max(m, b.count), 0);

  return (
    <div className="rounded-lg border border-slate-200 bg-white">
      <div className="flex items-baseline justify-between px-4 py-2 border-b border-slate-100">
        <h2 className="text-sm font-semibold text-slate-700">Распределение скоров квалификации</h2>
        <span className="text-xs text-slate-400">всего: {data.total}</span>
      </div>
      {data.total === 0 ? (
        <p className="p-6 text-center text-sm text-slate-500">Пока нет квалифицированных лидов.</p>
      ) : (
        <ul className="divide-y divide-slate-100">
          {data.buckets.map((b) => (
            <li key={b.lo} className="flex items-center gap-3 px-4 py-2">
              <span className="w-16 shrink-0 text-xs font-medium text-slate-600 tabular-nums">{b.label}</span>
              <div className="h-4 flex-1 overflow-hidden rounded bg-slate-100">
                <div
                  className="h-full bg-[#3b6ef6]"
                  style={{ width: maxCount > 0 ? `${(b.count / maxCount) * 100}%` : "0%" }}
                />
              </div>
              <span className="w-10 shrink-0 text-right text-xs tabular-nums text-slate-700">{b.count}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
