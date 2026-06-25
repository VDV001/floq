"use client";

import { useFunnelAnalytics } from "@/hooks/useFunnelAnalytics";
import { AnalyticsTabs } from "@/components/analytics/AnalyticsTabs";
import { QualDistributionCard } from "@/components/analytics/QualDistributionCard";
import { SequenceConversionTable } from "@/components/analytics/SequenceConversionTable";

export default function FunnelAnalyticsPage() {
  const { distribution, conversion, loading, error, lastUpdated, refresh } = useFunnelAnalytics();

  return (
    <section className="flex-1 overflow-y-auto px-4 sm:px-8 lg:px-12 py-8">
      <div className="mx-auto max-w-6xl space-y-6">
        <AnalyticsTabs />
        <header className="flex items-end justify-between gap-4 flex-wrap">
          <div>
            <h1 className="text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">Воронка</h1>
            <p className="text-sm text-slate-500 mt-1">
              Распределение скоров квалификации и конверсия по шагам секвенций (за всё время).
            </p>
          </div>
          <button
            type="button"
            onClick={() => void refresh()}
            className="rounded-md border border-slate-200 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
          >
            Обновить
          </button>
        </header>

        {error && (
          <div role="alert" className="rounded-md border border-red-200 bg-red-50 px-4 py-2 text-sm text-red-700">
            Не удалось загрузить данные: {error.message}
          </div>
        )}

        {loading && !distribution && !conversion ? (
          <div className="rounded-lg border border-slate-200 bg-white p-8 text-center text-sm text-slate-500">
            Загружаем…
          </div>
        ) : (
          <>
            {distribution && <QualDistributionCard data={distribution} />}
            {conversion && <SequenceConversionTable data={conversion} />}
          </>
        )}

        {lastUpdated && (
          <div className="text-xs text-slate-400">
            Обновлено: {lastUpdated.toLocaleTimeString("ru-RU")}
          </div>
        )}
      </div>
    </section>
  );
}
