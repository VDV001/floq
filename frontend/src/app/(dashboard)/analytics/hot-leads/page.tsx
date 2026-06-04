"use client";

import { useState } from "react";
import type { AnalyticsPeriod, ChannelFilter, LeadStatusFilter } from "@/lib/api";
import { useHotLeads } from "@/hooks/useHotLeads";
import { AnalyticsTabs } from "@/components/analytics/AnalyticsTabs";
import { PeriodSelector } from "@/components/analytics/PeriodSelector";
import { HotLeadsFilterBar } from "@/components/analytics/HotLeadsFilterBar";
import { HotLeadsTable } from "@/components/analytics/HotLeadsTable";

export default function HotLeadsAnalyticsPage() {
  const [period, setPeriod] = useState<AnalyticsPeriod>("all");
  const [status, setStatus] = useState<LeadStatusFilter>("any");
  const [channel, setChannel] = useState<ChannelFilter>("any");

  const { leads, totalMatching, loading, error, lastUpdated, refresh } = useHotLeads({
    period,
    status,
    channel,
  });

  return (
    <section className="flex-1 overflow-y-auto px-4 sm:px-8 lg:px-12 py-8">
      <div className="mx-auto max-w-6xl space-y-6">
        <AnalyticsTabs />
        <header className="flex items-end justify-between gap-4 flex-wrap">
          <div>
            <h1 className="text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">Горячие лиды</h1>
            <p className="text-sm text-slate-500 mt-1">
              Лиды по убыванию скора квалификации — кому звонить первым.
            </p>
          </div>
          <div className="flex items-center gap-3 flex-wrap">
            <HotLeadsFilterBar
              status={status}
              channel={channel}
              onStatusChange={setStatus}
              onChannelChange={setChannel}
            />
            <PeriodSelector value={period} onChange={setPeriod} />
            <button
              type="button"
              onClick={() => void refresh()}
              className="rounded-md border border-slate-200 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
            >
              Обновить
            </button>
          </div>
        </header>

        {error && (
          <div role="alert" className="rounded-md border border-red-200 bg-red-50 px-4 py-2 text-sm text-red-700">
            Не удалось загрузить данные: {error.message}
          </div>
        )}

        {loading && leads.length === 0 ? (
          <div className="rounded-lg border border-slate-200 bg-white p-8 text-center text-sm text-slate-500">
            Загружаем…
          </div>
        ) : (
          <>
            <HotLeadsTable leads={leads} />
            {leads.length > 0 && (
              <div className="text-xs text-slate-400">
                Показано {leads.length} из {totalMatching}
                {lastUpdated && <> · обновлено {lastUpdated.toLocaleTimeString("ru-RU")}</>}
              </div>
            )}
          </>
        )}
      </div>
    </section>
  );
}
