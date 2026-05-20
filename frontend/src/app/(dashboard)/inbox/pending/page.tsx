"use client";

import { useState } from "react";
import { ShieldCheck } from "lucide-react";
import { FilterPill } from "@/components/outbound/FilterPill";
import { formatTime } from "@/components/outbound/constants";
import { PendingQueueRow } from "@/components/pending-queue/PendingQueueRow";
import { PendingQueueTabs } from "@/components/pending-queue/PendingQueueTabs";
import { usePendingQueue } from "@/hooks/usePendingQueue";

export default function InboxPendingPage() {
  const q = usePendingQueue();
  const [busyId, setBusyId] = useState<string | null>(null);

  async function withBusy(id: string, op: (id: string) => Promise<void>) {
    setBusyId(id);
    try {
      await op(id);
    } finally {
      setBusyId(null);
    }
  }

  return (
    <section className="flex-1 overflow-y-auto px-4 sm:px-8 lg:px-12 py-8">
      <div className="mx-auto max-w-4xl">
        <div className="mb-6 flex items-end justify-between">
          <div>
            <div className="flex items-center gap-3">
              <h2 className="text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">
                Очередь HITL
              </h2>
              {q.loading && (
                <div className="size-5 animate-spin rounded-full border-2 border-[#3b6ef6] border-t-transparent" />
              )}
            </div>
            <p className="mt-1 text-sm text-[#434655]">
              Auto-черновики, ожидающие вашего одобрения перед отправкой
            </p>
          </div>
          <span className="text-[11px] text-[#737686]">
            Обновлено: {formatTime(q.lastUpdated)}
          </span>
        </div>

        <PendingQueueTabs pendingCount={q.rows.length} />

        <div className="mb-6 flex flex-wrap items-center gap-2">
          <span className="mr-2 text-[11px] font-bold uppercase tracking-wider text-[#737686]">
            Канал
          </span>
          <FilterPill label="Все" value="all" current={q.channelFilter} onChange={q.setChannelFilter} />
          <FilterPill label="Telegram" value="telegram" current={q.channelFilter} onChange={q.setChannelFilter} />
          <FilterPill label="Email" value="email" current={q.channelFilter} onChange={q.setChannelFilter} />
          <span className="ml-6 mr-2 text-[11px] font-bold uppercase tracking-wider text-[#737686]">
            Тип
          </span>
          <FilterPill label="Все" value="all" current={q.kindFilter} onChange={q.setKindFilter} />
          <FilterPill label="Встреча" value="booking_link" current={q.kindFilter} onChange={q.setKindFilter} />
        </div>

        {!q.loading && q.filtered.length === 0 && (
          <div className="flex flex-col items-center justify-center rounded-2xl bg-white py-16 text-center">
            <ShieldCheck className="mb-4 size-10 text-[#c3c6d7]" />
            <p className="text-lg font-semibold text-[#434655]">
              {q.rows.length === 0 ? "Нет драфтов на одобрение" : "Под фильтр ничего не попало"}
            </p>
            <p className="mt-1 text-sm text-[#737686]">
              {q.rows.length === 0
                ? "AI-черновики появляются здесь, как только подходящее сообщение прилетает от лида"
                : "Снимите фильтр канала или типа, чтобы увидеть весь список"}
            </p>
          </div>
        )}

        <ul className="space-y-3">
          {q.filtered.map((row) => (
            <PendingQueueRow
              key={row.id}
              row={row}
              busy={busyId === row.id}
              onApprove={(id) => withBusy(id, q.handleApprove)}
              onReject={(id) => withBusy(id, q.handleReject)}
            />
          ))}
        </ul>
      </div>
    </section>
  );
}
