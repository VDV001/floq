"use client";

import { useState, useEffect } from "react";
import { api, Lead } from "@/lib/api";
import {
  Clock,
  Send,
  Sparkles,
  X,
  Mail,
  Calendar,
  Filter,
  Archive,
  Lightbulb,
} from "lucide-react";

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function getTimeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "Только что";
  if (mins < 60) return `${mins} мин`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours} ч`;
  const days = Math.floor(hours / 24);
  return `${days} д`;
}

function getInitials(name: string): string {
  return name
    .split(" ")
    .map((w) => w[0])
    .join("")
    .slice(0, 2)
    .toUpperCase();
}

function getSilentDays(dateStr: string): number {
  const diff = Date.now() - new Date(dateStr).getTime();
  return Math.max(1, Math.floor(diff / (1000 * 60 * 60 * 24)));
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function AlertsPage() {
  const [leads, setLeads] = useState<Lead[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchLeads = () => {
      api.getLeads().then((data) => setLeads(data)).catch(() => {}).finally(() => setLoading(false));
    };
    fetchLeads();
    const interval = setInterval(fetchLeads, 5000);
    return () => clearInterval(interval);
  }, []);

  const followupLeads = leads.filter((l) => l.status === "followup");
  const featured = followupLeads[0] ?? null;
  const listAlerts = followupLeads.slice(1);

  const totalLeads = leads.length;
  const criticalCount = followupLeads.filter(
    (l) => getSilentDays(l.updated_at) >= 4
  ).length;
  const warningCount = followupLeads.length - criticalCount;

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
        {/* Featured Card (2 cols) */}
        {featured && (
          <div className="xl:col-span-2">
            <div className="relative overflow-hidden rounded-xl bg-white p-8 transition-all duration-300 hover:shadow-[0_12px_40px_rgba(13,28,46,0.06)]">
              {/* Silent badge */}
              <div className="absolute right-6 top-6">
                <span className="flex items-center gap-1 text-[0.7rem] font-black uppercase tracking-tight text-[#ba1a1a]">
                  <Clock className="size-3.5" />
                  Молчит {getSilentDays(featured.updated_at)} д
                </span>
              </div>

              {/* Contact info */}
              <div className="mb-8 flex items-start gap-6">
                <div className="flex size-16 shrink-0 items-center justify-center rounded-2xl bg-[#d5e0f8] text-lg font-black text-[#434655] shadow-md">
                  {getInitials(featured.contact_name)}
                </div>
                <div>
                  <h3 className="mb-1 text-2xl font-bold text-[#0d1c2e]">
                    {featured.contact_name}
                  </h3>
                  <p className="flex items-center text-sm font-medium text-[#434655]">
                    {featured.company || "—"} ·{" "}
                    {featured.channel === "telegram" ? "Telegram" : "Email"}
                  </p>
                </div>
              </div>

              {/* AI Suggestion */}
              <div className="relative mb-8 rounded-xl bg-[#e1e0ff] p-6">
                <div className="absolute -top-3 left-6 flex items-center gap-1 rounded-md bg-[#3e3fcc] px-3 py-1 text-[0.6rem] font-bold text-white">
                  <Sparkles className="size-3" />
                  ИИ РЕКОМЕНДАЦИЯ
                </div>
                <p className="mb-2 text-lg font-bold text-[#2f2ebe]">
                  &laquo;Напомните о диалоге, который начался{" "}
                  {getTimeAgo(featured.created_at)} назад.&raquo;
                </p>
                <p className="text-sm italic text-[#2f2ebe]/70">
                  {featured.first_message
                    ? `Последнее сообщение: "${featured.first_message.slice(0, 120)}${featured.first_message.length > 120 ? "..." : ""}"`
                    : "Floq рекомендует связаться с лидом для сохранения импульса."}
                </p>
              </div>

              {/* Actions */}
              <div className="flex items-center justify-between pt-4">
                <div className="flex gap-3">
                  <button className="flex items-center gap-2 rounded-lg bg-gradient-to-r from-[#004ac6] to-[#2563eb] px-6 py-3 font-bold text-white shadow-md transition-all hover:opacity-90">
                    <Send className="size-4" />
                    Отправить напоминание
                  </button>
                  <button className="rounded-lg bg-[#eff4ff] px-6 py-3 font-bold text-[#434655] transition-all hover:bg-[#dce9ff]">
                    Отложить
                  </button>
                </div>
                <button className="flex items-center gap-1 text-sm font-bold text-[#434655] transition-colors hover:text-[#ba1a1a]">
                  <X className="size-[18px]" />
                  Закрыть
                </button>
              </div>
            </div>
          </div>
        )}

        {/* Side Stats (1 col) */}
        <div className="space-y-8">
          {/* Alert Summary */}
          <div className="rounded-xl bg-[#eff4ff] p-6">
            <h4 className="mb-6 text-sm font-bold uppercase tracking-widest text-[#0d1c2e]">
              Сводка алертов
            </h4>
            <div className="space-y-6">
              <div>
                <div className="mb-2 flex items-center justify-between">
                  <span className="text-sm font-medium text-[#434655]">
                    Критические (4д+)
                  </span>
                  <span className="font-extrabold text-[#ba1a1a]">
                    {criticalCount}
                  </span>
                </div>
                <div className="h-1 overflow-hidden rounded-full bg-[#dce9ff]">
                  <div
                    className="h-full bg-[#ba1a1a]"
                    style={{
                      width:
                        followupLeads.length > 0
                          ? `${(criticalCount / followupLeads.length) * 100}%`
                          : "0%",
                    }}
                  />
                </div>
              </div>
              <div>
                <div className="mb-2 flex items-center justify-between">
                  <span className="text-sm font-medium text-[#434655]">
                    Предупреждения (2д)
                  </span>
                  <span className="font-extrabold text-[#004ac6]">
                    {warningCount}
                  </span>
                </div>
                <div className="h-1 overflow-hidden rounded-full bg-[#dce9ff]">
                  <div
                    className="h-full bg-[#004ac6]"
                    style={{
                      width:
                        followupLeads.length > 0
                          ? `${(warningCount / followupLeads.length) * 100}%`
                          : "0%",
                    }}
                  />
                </div>
              </div>
            </div>
          </div>

          {/* Insight card */}
          <div className="rounded-xl border border-[#c3c6d7]/10 bg-white p-6 shadow-sm">
            <div className="mb-4 flex items-center gap-3">
              <div className="flex size-8 items-center justify-center rounded-lg bg-[#e1e0ff]">
                <Lightbulb className="size-4 text-[#3e3fcc]" />
              </div>
              <p className="text-sm font-bold text-[#0d1c2e]">Знаете ли вы?</p>
            </div>
            <p className="text-xs leading-relaxed text-[#434655]">
              Лиды, которым напомнили в течение 48 часов,{" "}
              <span className="font-bold text-[#0d1c2e]">
                в 3.4 раза чаще
              </span>{" "}
              конвертируются в закрытые сделки.
            </p>
          </div>
        </div>

        {/* List Items (full width) */}
        <div className="space-y-4 xl:col-span-3">
          {listAlerts.map((alert) => (
            <div
              key={alert.id}
              className="flex flex-wrap items-center justify-between rounded-xl bg-white p-6 transition-all duration-300 hover:bg-[#eff4ff]"
            >
              {/* Left: contact */}
              <div className="flex min-w-[300px] flex-1 items-center gap-5">
                <div className="flex size-12 shrink-0 items-center justify-center rounded-xl bg-[#d5e0f8] font-black text-[#434655]">
                  {getInitials(alert.contact_name)}
                </div>
                <div>
                  <h4 className="text-lg font-bold text-[#0d1c2e]">
                    {alert.contact_name}
                  </h4>
                  <p className="text-sm font-medium text-[#434655]">
                    {alert.company || "—"} ·{" "}
                    {alert.channel === "telegram" ? "Telegram" : "Email"}
                  </p>
                </div>
                <div className="ml-4 hidden border-l border-[#c3c6d7]/20 px-4 py-2 md:block">
                  <p className="mb-1 text-[0.65rem] font-bold uppercase tracking-widest text-[#434655]">
                    Последний контакт
                  </p>
                  <p className="text-sm font-bold text-[#0d1c2e]">
                    {getTimeAgo(alert.updated_at)} назад
                  </p>
                </div>
              </div>

              {/* Middle: first message preview */}
              <div className="hidden min-w-[400px] flex-1 px-8 lg:block">
                <div className="flex items-center gap-3">
                  <Sparkles className="size-5 shrink-0 text-[#3e3fcc]" />
                  <p className="text-sm font-semibold text-[#434655]">
                    <span className="font-bold text-[#0d1c2e]">Действие:</span>{" "}
                    {alert.first_message
                      ? `Напомнить о: "${alert.first_message.slice(0, 80)}${alert.first_message.length > 80 ? "..." : ""}"`
                      : "Связаться с лидом для продолжения диалога."}
                  </p>
                </div>
              </div>

              {/* Right: buttons */}
              <div className="flex items-center gap-2">
                <button className="flex size-10 items-center justify-center rounded-lg bg-[#2563eb] text-white shadow-sm transition-all hover:scale-105">
                  <Mail className="size-[18px]" />
                </button>
                <button className="flex size-10 items-center justify-center rounded-lg text-[#434655] transition-all hover:bg-white hover:shadow-sm">
                  <Calendar className="size-[18px]" />
                </button>
                <button className="flex size-10 items-center justify-center rounded-lg text-[#434655] transition-all hover:text-[#ba1a1a]">
                  <X className="size-[18px]" />
                </button>
              </div>
            </div>
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
