"use client";

import { Filter, Archive } from "lucide-react";
import { useAlerts } from "@/hooks/useAlerts";
import { FeaturedCard } from "@/components/alerts/FeaturedCard";
import { AlertSummary } from "@/components/alerts/AlertSummary";
import { AlertListItem } from "@/components/alerts/AlertListItem";

export default function AlertsPage() {
  const {
    loading,
    followupLeads,
    featured,
    listAlerts,
    totalLeads,
    criticalCount,
    warningCount,
  } = useAlerts();

  if (loading) {
    return (
      <div className="flex min-h-full items-center justify-center">
        <div className="size-8 animate-spin rounded-full border-4 border-[#3b6ef6] border-t-transparent" />
      </div>
    );
  }

  if (followupLeads.length === 0) {
    return (
      <div className="flex min-h-full items-center justify-center p-4 sm:p-8 lg:p-12">
        <div className="text-center">
          <p className="text-2xl font-bold text-[#0d1c2e]">Нет напоминаний</p>
          <p className="mt-2 text-sm text-[#434655]">
            Все лиды в работе, фоллоуапов нет.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-full p-4 sm:p-8 lg:p-12">
      {/* Header */}
      <header className="mb-12 flex items-end justify-between">
        <div className="max-w-2xl">
          <div className="mb-4 flex items-center gap-2">
            <span className="rounded-full bg-[#e1e0ff] px-3 py-1 text-[0.65rem] font-black uppercase tracking-widest text-[#07006c]">
              Высокий приоритет
            </span>
            <span className="size-1.5 animate-pulse rounded-full bg-[#ba1a1a]" />
          </div>
          <h2 className="mb-4 text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">
            Напоминания
          </h2>
          <p className="text-lg font-medium text-[#434655]">
            Floq обнаружил{" "}
            <span className="font-bold text-[#004ac6]">
              {followupLeads.length} лидов
            </span>
            , которые требуют фоллоуапа. Рекомендуется срочное действие для
            сохранения импульса.
          </p>
        </div>
        <div className="flex gap-3">
          <button className="flex items-center gap-2 rounded-xl bg-white px-5 py-3 text-sm font-bold text-[#434655] shadow-sm transition-all hover:bg-[#eff4ff]">
            <Filter className="size-[18px]" />
            Фильтр
          </button>
          <button className="flex items-center gap-2 rounded-xl bg-white px-5 py-3 text-sm font-bold text-[#434655] shadow-sm transition-all hover:bg-[#eff4ff]">
            <Archive className="size-[18px]" />
            Очистить все
          </button>
        </div>
      </header>

      {/* Bento Grid */}
      <div className="grid grid-cols-1 gap-8 xl:grid-cols-3">
        {featured && <FeaturedCard featured={featured} />}

        <AlertSummary
          followupCount={followupLeads.length}
          criticalCount={criticalCount}
          warningCount={warningCount}
        />

        {/* List Items (full width) */}
        <div className="space-y-4 xl:col-span-3">
          {listAlerts.map((alert) => (
            <AlertListItem key={alert.id} alert={alert} />
          ))}
        </div>
      </div>

      {/* Footer Stats */}
      <footer className="mt-20 flex flex-wrap items-center justify-between border-t border-[#c3c6d7]/10 pt-10 text-[#434655]">
        <div className="flex items-center gap-8">
          <div>
            <p className="mb-1 text-[0.65rem] font-black uppercase tracking-widest">
              Всего лидов
            </p>
            <p className="text-2xl font-bold text-[#0d1c2e]">{totalLeads}</p>
          </div>
          <div>
            <p className="mb-1 text-[0.65rem] font-black uppercase tracking-widest">
              Требуют фоллоуапа
            </p>
            <p className="text-2xl font-bold text-[#0d1c2e]">
              {followupLeads.length}
            </p>
          </div>
        </div>
        <p className="text-sm font-medium">
          Сгенерировано{" "}
          <span className="font-bold text-[#004ac6]">Floq AI</span>
        </p>
      </footer>
    </div>
  );
}
