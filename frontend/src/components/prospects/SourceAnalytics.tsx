import { BarChart3 } from "lucide-react";
import type { SourceStatItem } from "@/lib/api";

export function SourceAnalytics({ stats }: { stats: SourceStatItem[] }) {
  const filtered = stats.filter((s) => s.prospect_count > 0 || s.lead_count > 0);
  if (filtered.length === 0) return null;

  return (
    <div className="rounded-xl border border-[#c3c6d7]/10 bg-white p-6 shadow-sm">
      <h4 className="mb-4 flex items-center gap-2 text-sm font-bold text-[#0d1c2e]">
        <BarChart3 className="size-4" />
        Конверсия по источникам
      </h4>
      <div className="space-y-3">
        {filtered.map((s) => {
          const total = s.prospect_count + s.lead_count;
          const convRate = total > 0 ? Math.round((s.converted_count / total) * 100) : 0;
          return (
            <div key={s.source_id}>
              <div className="mb-1 flex items-center justify-between">
                <span className="text-xs font-semibold text-[#0d1c2e]">{s.source_name}</span>
                <span className="text-[10px] font-medium text-[#737686]">{s.category_name}</span>
              </div>
              <div className="flex items-center gap-3">
                <div className="h-2 flex-1 overflow-hidden rounded-full bg-[#eff4ff]">
                  <div className="h-full rounded-full bg-[#004ac6] transition-all" style={{ width: `${Math.min(100, convRate)}%` }} />
                </div>
                <span className="w-8 text-right text-[10px] font-bold text-[#004ac6]">{convRate}%</span>
              </div>
              <div className="mt-0.5 flex gap-3 text-[10px] text-[#737686]">
                <span>{s.prospect_count} просп.</span>
                <span>{s.lead_count} лидов</span>
                <span>{s.converted_count} конв.</span>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
