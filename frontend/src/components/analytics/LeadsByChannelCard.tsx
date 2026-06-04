"use client";

interface LeadsByChannelCardProps {
  byChannel: Record<string, number>;
  total: number;
}

// CHANNELS is the fixed set of inbound channels (the lead_channel enum),
// rendered in a stable order so a channel with zero leads in the period
// still shows as a 0 row rather than vanishing.
const CHANNELS: { key: string; label: string; color: string }[] = [
  { key: "telegram", label: "Telegram", color: "bg-sky-500" },
  { key: "email", label: "Email", color: "bg-amber-500" },
];

// LeadsByChannelCard shows the inbound split across channels with each
// channel's count and share of the total. A proportional bar gives the
// donut-like glance the dashboard wants without a charting dependency.
export function LeadsByChannelCard({ byChannel, total }: LeadsByChannelCardProps) {
  return (
    <div className="rounded-lg border border-slate-200 bg-white p-6">
      <div className="flex items-baseline justify-between">
        <h2 className="text-sm font-semibold text-slate-700">Лиды по каналам</h2>
        <span className="text-sm text-slate-500 tabular-nums">{total}</span>
      </div>
      {total === 0 ? (
        <p className="mt-4 text-center text-sm text-slate-500">Нет лидов за период.</p>
      ) : (
        <div className="mt-4 space-y-3">
          {CHANNELS.map((ch) => {
            const count = byChannel[ch.key] ?? 0;
            const pct = total > 0 ? Math.round((count / total) * 100) : 0;
            return (
              <div key={ch.key} data-testid={`channel-${ch.key}`} className="text-sm">
                <div className="flex items-center justify-between">
                  <span className="font-medium text-slate-700">{ch.label}</span>
                  <span className="tabular-nums text-slate-900">
                    {count} <span className="text-slate-400">· {pct}%</span>
                  </span>
                </div>
                <div className="mt-1 h-2 overflow-hidden rounded bg-slate-100">
                  <div className={`h-full rounded ${ch.color}`} style={{ width: `${pct}%` }} />
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
