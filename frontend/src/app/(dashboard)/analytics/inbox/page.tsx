"use client";

import { useState } from "react";
import type { AnalyticsPeriod } from "@/lib/api";
import { useInboxAnalytics } from "@/hooks/useInboxAnalytics";
import { AnalyticsTabs } from "@/components/analytics/AnalyticsTabs";
import { PeriodSelector } from "@/components/analytics/PeriodSelector";
import { LeadsByChannelCard } from "@/components/analytics/LeadsByChannelCard";
import { LeadsByStatusTable } from "@/components/analytics/LeadsByStatusTable";
import { QualificationHistogram } from "@/components/analytics/QualificationHistogram";
import { PendingRepliesSummary } from "@/components/analytics/PendingRepliesSummary";

export default function InboxAnalyticsPage() {
  const [period, setPeriod] = useState<AnalyticsPeriod>("month");
  const { data, loading, error, lastUpdated, refresh } = useInboxAnalytics(period);

  return (
    <section className="flex-1 overflow-y-auto px-4 sm:px-8 lg:px-12 py-8">
      <div className="mx-auto max-w-6xl space-y-6">
        <AnalyticsTabs />
        <header className="flex items-end justify-between gap-4 flex-wrap">
          <div>
            <h1 className="text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">Входящие</h1>
            <p className="text-sm text-slate-500 mt-1">
              Поток входящих лидов: каналы, статусы, квалификация и AI-черновики.
            </p>
          </div>
          <div className="flex items-center gap-3 flex-wrap">
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

        {loading && !data ? (
          <div className="rounded-lg border border-slate-200 bg-white p-8 text-center text-sm text-slate-500">
            Загружаем…
          </div>
        ) : data ? (
          <>
            <div className="grid gap-4 lg:grid-cols-2">
              <LeadsByChannelCard byChannel={data.leads.by_channel} total={data.leads.total} />
              <LeadsByStatusTable byStatus={data.leads.by_status} total={data.leads.total} />
            </div>
            <div className="grid gap-4 lg:grid-cols-2">
              <QualificationHistogram
                histogram={data.qualifications.score_histogram}
                avgScore={data.qualifications.avg_score}
              />
              <PendingRepliesSummary stats={data.pending_replies} />
            </div>
            {lastUpdated && (
              <div className="text-xs text-slate-400">
                Обновлено {lastUpdated.toLocaleTimeString("ru-RU")}
              </div>
            )}
          </>
        ) : null}
      </div>
    </section>
  );
}
