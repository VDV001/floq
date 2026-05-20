"use client";

import type { CostRatiosResponse } from "@/lib/api";

interface CostRatiosPanelProps {
  ratios: CostRatiosResponse;
}

function formatUSD(v: number): string {
  if (v === 0) return "—";
  if (v < 0.01) return `$${v.toFixed(4)}`;
  return `$${v.toFixed(2)}`;
}

function RatioCard({ label, ratio, count, hint }: { label: string; ratio: number; count: number; hint: string }) {
  return (
    <div className="rounded-lg border border-slate-200 bg-white p-4">
      <div className="text-xs uppercase tracking-wide text-slate-500">{label}</div>
      <div className="mt-2 text-2xl font-extrabold tabular-nums text-[#0d1c2e]">{formatUSD(ratio)}</div>
      <div className="mt-1 text-xs text-slate-400">
        {count > 0 ? `на ${count} ${hint}` : `нет ${hint} в периоде`}
      </div>
    </div>
  );
}

export function CostRatiosPanel({ ratios }: CostRatiosPanelProps) {
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
      <RatioCard
        label="Стоимость / лид"
        ratio={ratios.cost_per_lead_usd}
        count={ratios.leads_count}
        hint="лидов"
      />
      <RatioCard
        label="Стоимость / квалиф. лид"
        ratio={ratios.cost_per_qualified_lead_usd}
        count={ratios.qualified_leads_count}
        hint="квалиф. лидов"
      />
      <RatioCard
        label="Стоимость / конверсия"
        ratio={ratios.cost_per_converted_usd}
        count={ratios.converted_count}
        hint="конверсий"
      />
      <RatioCard
        label="Стоимость / отправл."
        ratio={ratios.cost_per_draft_sent_usd}
        count={ratios.drafts_sent_count}
        hint="отправленных"
      />
    </div>
  );
}
