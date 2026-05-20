"use client";

import type { CostRatiosResponse } from "@/lib/api";

interface CostSummaryCardProps {
  ratios: CostRatiosResponse;
}

function formatUSD(v: number): string {
  if (v < 0.01) return `$${v.toFixed(4)}`;
  return `$${v.toFixed(2)}`;
}

export function CostSummaryCard({ ratios }: CostSummaryCardProps) {
  return (
    <div className="rounded-lg border border-slate-200 bg-white p-5">
      <div className="flex items-baseline justify-between gap-4">
        <div>
          <div className="text-xs uppercase tracking-wide text-slate-500">AI-расход за период</div>
          <div className="mt-2 text-4xl font-extrabold tabular-nums text-[#0d1c2e]">
            {formatUSD(ratios.total_cost_usd)}
          </div>
        </div>
        <div className="text-right">
          <div className="text-xs uppercase tracking-wide text-slate-500">Вызовов</div>
          <div className="mt-2 text-2xl font-bold tabular-nums text-slate-700">{ratios.total_calls}</div>
        </div>
      </div>
    </div>
  );
}
