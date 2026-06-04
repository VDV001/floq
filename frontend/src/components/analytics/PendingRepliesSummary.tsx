"use client";

import type { InboxFlowResponse } from "@/lib/api";

interface PendingRepliesSummaryProps {
  stats: InboxFlowResponse["pending_replies"];
}

// formatDecideDuration renders a time-to-decide value (whole seconds)
// for the operator dashboard. Zero means "no decided rows" and shows a
// dash rather than "0 с" — a real decision is never instantaneous, so 0
// is the empty sentinel, not a measurement.
export function formatDecideDuration(seconds: number): string {
  if (seconds <= 0) return "—";
  if (seconds < 60) return `${seconds} с`;
  const mins = Math.floor(seconds / 60);
  const rem = seconds % 60;
  return rem === 0 ? `${mins} мин` : `${mins} мин ${rem} с`;
}

function Metric({ label, value, testId }: { label: string; value: string; testId: string }) {
  return (
    <div className="rounded-md border border-slate-100 bg-slate-50 px-3 py-2">
      <div className="text-xs text-slate-500">{label}</div>
      <div data-testid={testId} className="mt-0.5 text-lg font-semibold tabular-nums text-slate-900">
        {value}
      </div>
    </div>
  );
}

// PendingRepliesSummary shows the HITL approval health: the approve rate
// (operator approvals over decisions made), the approved/rejected/pending
// breakdown, and the p50/p95 time-to-decide. The approve rate arrives
// pre-computed from the backend (0 on no decisions) so this component
// only formats.
export function PendingRepliesSummary({ stats }: PendingRepliesSummaryProps) {
  const approvePct = Math.round(stats.approve_rate * 100);
  return (
    <div className="rounded-lg border border-slate-200 bg-white p-6">
      <div className="flex items-baseline justify-between">
        <h2 className="text-sm font-semibold text-slate-700">AI-черновики (HITL)</h2>
        <span className="text-sm text-slate-500">
          Одобрено: <span className="font-semibold text-slate-900 tabular-nums">{approvePct}%</span>
        </span>
      </div>
      <div className="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-3">
        <Metric label="Одобрено" value={String(stats.approved)} testId="pr-approved" />
        <Metric label="Отклонено" value={String(stats.rejected)} testId="pr-rejected" />
        <Metric label="В очереди" value={String(stats.currently_pending)} testId="pr-pending" />
        <Metric label="Решение (p50)" value={formatDecideDuration(stats.p50_time_to_decide_seconds)} testId="pr-p50" />
        <Metric label="Решение (p95)" value={formatDecideDuration(stats.p95_time_to_decide_seconds)} testId="pr-p95" />
      </div>
    </div>
  );
}
