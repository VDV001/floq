import { cn } from "@/lib/utils";
import type { PipelineColumn } from "./constants";
import { LeadCard } from "./LeadCard";

export function KanbanColumn({ column }: { column: PipelineColumn }) {
  return (
    <div className="flex min-w-[280px] shrink-0 flex-col">
      <div className="mb-3 flex items-center gap-2">
        <span className="size-2.5 rounded-full" style={{ backgroundColor: column.dotColor }} />
        <span className="text-sm font-semibold text-[#0d1c2e]">{column.title}</span>
        <span className={cn("rounded-full px-2 py-0.5 text-xs font-medium", column.badgeStyle)}>{column.count}</span>
      </div>
      <div className="flex flex-col gap-3">
        {column.leads.map((lead) => <LeadCard key={lead.id} lead={lead} />)}
      </div>
    </div>
  );
}
