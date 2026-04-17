"use client";

import { cn } from "@/lib/utils";
import { CHANNEL_FILTERS } from "@/components/pipeline/constants";
import { KanbanColumn } from "@/components/pipeline/KanbanColumn";
import { MetricCards } from "@/components/pipeline/MetricCards";
import { usePipelinePage } from "@/hooks/usePipelinePage";

export default function PipelinePage() {
  const { activeChannel, setActiveChannel, loading, totalActive, columnCounts, filteredColumns } = usePipelinePage();

  return (
    <div className="flex h-full flex-col bg-[#f8f9ff]">
      <div className="border-b border-[#c3c6d7]/10 bg-white/60 px-6 pb-4 pt-6 backdrop-blur">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <div className="flex items-center gap-3">
              <h1 className="text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">Воронка продаж</h1>
              {loading && <div className="size-5 animate-spin rounded-full border-2 border-[#3b6ef6] border-t-transparent" />}
            </div>
            <p className="mt-1 text-sm text-[#434655]">Управление лидами в реальном времени с поддержкой Floq AI</p>
          </div>
          <div className="flex items-center gap-1 rounded-xl bg-[#eff4ff] p-1">
            {CHANNEL_FILTERS.map((f) => (
              <button key={f.value} onClick={() => setActiveChannel(f.value)}
                className={cn("rounded-lg px-3.5 py-1.5 text-sm font-medium transition-colors",
                  activeChannel === f.value ? "bg-white text-[#0d1c2e] shadow" : "text-[#434655] hover:text-[#0d1c2e]"
                )}>{f.label}</button>
            ))}
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-auto p-6">
        <MetricCards totalActive={totalActive} columnCounts={columnCounts} />
        <div className="flex gap-6 overflow-x-auto pb-4">
          {filteredColumns.map((col) => <KanbanColumn key={col.key} column={col} />)}
        </div>
      </div>
    </div>
  );
}
