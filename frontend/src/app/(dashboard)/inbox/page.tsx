"use client";

import { useState, useEffect } from "react";
import { api } from "@/lib/api";
import Link from "next/link";
import {
  Mail,
  Send,
  Sparkles,
  Star,
  CheckCircle2,
  MessageCircle,
  RotateCcw,
  CircleCheck,
} from "lucide-react";
import { cn } from "@/lib/utils";

/* ------------------------------------------------------------------ */
/*  Types & styles                                                     */
/* ------------------------------------------------------------------ */

interface Lead {
  id: string;
  company: string;
  contact: string;
  channel: "email" | "telegram";
  preview: string;
  timeAgo: string;
  status: "Новый" | "Квалифицирован" | "В диалоге" | "Нужен фоллоуап" | "Закрыт";
  apiStatus: string;
}

const STATUS_STYLES: Record<Lead["status"], string> = {
  "Новый": "bg-[#dbe1ff] text-[#004ac6]",
  "Квалифицирован": "bg-[#e6fffa] text-[#006b5f]",
  "В диалоге": "bg-[#dbeafe] text-[#1e40af]",
  "Нужен фоллоуап": "bg-[#fffbeb] text-[#92400e]",
  "Закрыт": "bg-[#f3f4f6] text-[#6b7280]",
};

/* ------------------------------------------------------------------ */
/*  Pipeline stages                                                    */
/* ------------------------------------------------------------------ */

const PIPELINE_STAGES_CONFIG: { id: string; apiStatus: string; label: string; icon: typeof Star; alert?: boolean }[] = [
  { id: "new", apiStatus: "new", label: "Новые лиды", icon: Star },
  { id: "qualified", apiStatus: "qualified", label: "Квалифицированные", icon: CheckCircle2 },
  { id: "conversation", apiStatus: "in_conversation", label: "В диалоге", icon: MessageCircle },
  { id: "followup", apiStatus: "followup", label: "Фоллоуап", icon: RotateCcw, alert: true },
  { id: "closed", apiStatus: "closed", label: "Закрытые", icon: CircleCheck },
];

const FILTER_TABS = ["Все", "Непрочитанные", "Приоритетные"] as const;

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

function mapStatus(status: string): Lead["status"] {
  switch (status) {
    case "new": return "Новый";
    case "qualified": return "Квалифицирован";
    case "in_conversation": return "В диалоге";
    case "followup": return "Нужен фоллоуап";
    case "closed": return "Закрыт";
    default: return "Новый";
  }
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function InboxPage() {
  const [activeFilter, setActiveFilter] = useState<string>("Все");
  const [activeStage, setActiveStage] = useState("new");
  const [loading, setLoading] = useState(true);
  const [leads, setLeads] = useState<Lead[]>([]);
  const [statusCounts, setStatusCounts] = useState<Record<string, number>>({});

  useEffect(() => {
    const fetchLeads = () => {
      api
        .getLeads()
        .then((data) => {
          const mapped: Lead[] = data.map((l) => ({
            id: l.id,
            company: l.company || "—",
            contact: l.contact_name,
            channel: l.channel as "email" | "telegram",
            preview: l.first_message === "/start" ? "Загрузка..." : (l.first_message || "Нет сообщений"),
            timeAgo: getTimeAgo(l.created_at),
            status: mapStatus(l.status),
            apiStatus: l.status,
          }));
          setLeads(mapped);

          // Load qualifications for better previews
          data.forEach((l) => {
            if (l.first_message === "/start") {
              api.getQualification(l.id).then((q) => {
                if (q?.identified_need) {
                  setLeads((prev) => prev.map((lead) =>
                    lead.id === l.id ? { ...lead, preview: q.identified_need } : lead
                  ));
                }
              }).catch(() => {});
            }
          });

          const counts: Record<string, number> = {};
          for (const l of data) {
            counts[l.status] = (counts[l.status] || 0) + 1;
          }
          setStatusCounts(counts);
        })
        .catch(() => {})
        .finally(() => setLoading(false));
    };
    fetchLeads();
    const interval = setInterval(fetchLeads, 30000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="flex h-full">
      {/* ── Secondary Sidebar: Pipeline Stages ── */}
      <nav className="w-72 shrink-0 overflow-y-auto border-r border-[#c3c6d7]/10 bg-[#eff4ff]/50 px-6 py-8 space-y-10">
        {/* Pipeline Stages */}
        <section>
          <h3 className="mb-4 px-2 text-[0.7rem] font-bold uppercase tracking-widest text-[#737686]">
            Этапы воронки
          </h3>
          <div className="space-y-1">
            {PIPELINE_STAGES_CONFIG.map((stage) => {
              const Icon = stage.icon;
              const isActive = activeStage === stage.id;
              const count = statusCounts[stage.apiStatus] || 0;
              return (
                <button
                  key={stage.id}
                  onClick={() => setActiveStage(stage.id)}
                  className={cn(
                    "flex w-full items-center justify-between rounded-xl px-3 py-2.5 text-sm transition-all",
                    isActive
                      ? "bg-white font-bold text-[#004ac6] shadow-sm"
                      : "text-[#434655] hover:bg-[#dce9ff] group"
                  )}
                >
                  <div className="flex items-center gap-3">
                    <Icon className="size-5" />
                    <span>{stage.label}</span>
                  </div>
                  <span
                    className={cn(
                      "rounded-full px-2 py-0.5 text-[10px] font-semibold",
                      isActive
                        ? "bg-[#dbe1ff] text-[#004ac6]"
                        : stage.alert && count > 0
                          ? "bg-[#ffdad6] text-[#93000a]"
                          : "text-[#737686] group-hover:text-[#004ac6]"
                    )}
                  >
                    {count}
                  </span>
                </button>
              );
            })}
          </div>
        </section>

        {/* Channels */}
        <section>
          <h3 className="mb-4 px-2 text-[0.7rem] font-bold uppercase tracking-widest text-[#737686]">
            Каналы
          </h3>
          <div className="grid grid-cols-2 gap-2">
            <button className="flex items-center gap-2 rounded-lg border border-[#c3c6d7]/10 bg-white px-3 py-2 text-xs font-medium">
              <Send className="size-4 text-[#229ED9]" />
              Telegram
            </button>
            <button className="flex items-center gap-2 rounded-lg border border-[#c3c6d7]/10 bg-white px-3 py-2 text-xs font-medium">
              <Mail className="size-4 text-[#004ac6]" />
              Email
            </button>
          </div>
        </section>

        {/* AI Summary — dynamic */}
        <div className="relative overflow-hidden rounded-2xl border border-[#c0c1ff]/30 bg-[#e1e0ff]/40 p-5">
          <div className="absolute -right-4 -top-4 size-16 rounded-full bg-[#585be6]/10 blur-2xl" />
          <div className="mb-3 flex items-center gap-2">
            <Sparkles className="size-4 text-[#3e3fcc]" />
            <span className="text-xs font-bold text-[#2f2ebe]">ИИ-сводка</span>
          </div>
          <p className="text-xs italic leading-relaxed text-[#2f2ebe]/80">
            {leads.length === 0
              ? "Нет активных лидов. Подключите Telegram бот в настройках чтобы начать получать обращения."
              : `${leads.length} ${leads.length === 1 ? "лид" : "лидов"} в системе. ${
                  (statusCounts["new"] || 0) > 0
                    ? `${statusCounts["new"]} новых ожидают ответа.`
                    : (statusCounts["followup"] || 0) > 0
                      ? `${statusCounts["followup"]} требуют фоллоуапа.`
                      : "Все лиды в работе."
                }`}
          </p>
        </div>
      </nav>

      {/* ── Main Feed ── */}
      <section className="flex-1 overflow-y-auto px-4 sm:px-8 lg:px-12 py-8">
        <div className="mx-auto max-w-4xl space-y-8">
          {/* Feed Header */}
          <div className="flex items-end justify-between">
            <div>
              <div className="flex items-center gap-3">
                <h2 className="text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">
                  Лента лидов
                </h2>
                {loading && (
                  <div className="size-5 animate-spin rounded-full border-2 border-[#3b6ef6] border-t-transparent" />
                )}
              </div>
              <p className="mt-1 text-sm text-[#434655]">
                Показано {leads.length} активных лидов для{" "}
                <span className="font-bold">Новые лиды</span>
              </p>
            </div>
            <div className="flex items-center gap-1 rounded-lg bg-[#eff4ff] p-1">
              {FILTER_TABS.map((tab) => (
                <button
                  key={tab}
                  onClick={() => setActiveFilter(tab)}
                  className={cn(
                    "rounded-md px-4 py-1.5 text-xs font-medium transition-colors",
                    activeFilter === tab
                      ? "bg-white font-bold text-[#004ac6] shadow-sm"
                      : "text-[#434655] hover:bg-[#dce9ff]"
                  )}
                >
                  {tab}
                </button>
              ))}
            </div>
          </div>

          {/* Lead Cards */}
          <div className="space-y-3">
            {!loading && leads.length === 0 && (
              <div className="rounded-xl bg-white p-12 text-center">
                <p className="text-lg font-bold text-[#0d1c2e]">Нет лидов</p>
                <p className="mt-2 text-sm text-[#434655]">
                  Напишите вашему Telegram боту чтобы создать первый лид
                </p>
              </div>
            )}
            {leads.filter((lead) => {
              const stageConfig = PIPELINE_STAGES_CONFIG.find((s) => s.id === activeStage);
              return stageConfig ? lead.apiStatus === stageConfig.apiStatus : true;
            }).map((lead) => (
              <Link
                key={lead.id}
                href={`/inbox/${lead.id}`}
                className="group relative flex cursor-pointer rounded-xl border border-transparent bg-white p-5 transition-all hover:border-[#c3c6d7]/10 hover:bg-[#dce9ff]/40"
              >
                {/* Left: channel icon + content */}
                <div className="flex items-start gap-4 flex-1 min-w-0">
                  <div
                    className={cn(
                      "flex size-12 shrink-0 items-center justify-center rounded-xl",
                      lead.channel === "email"
                        ? "bg-[#dbe1ff]"
                        : "bg-[#d5e0f8]"
                    )}
                  >
                    {lead.channel === "email" ? (
                      <Mail className="size-5 text-[#004ac6]" />
                    ) : (
                      <Send className="size-5 text-[#229ED9]" />
                    )}
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="mb-0.5">
                      <h4 className="font-bold leading-none text-[#0d1c2e]">
                        {lead.company}
                      </h4>
                      <p className="mt-1 text-xs font-medium text-[#737686]">
                        {lead.channel === "email" ? "по email" : "через Telegram"}{" "}
                        · {lead.contact}
                      </p>
                    </div>
                    <p className="mt-2 line-clamp-2 text-sm leading-relaxed text-[#434655]">
                      {lead.preview}
                    </p>
                  </div>
                </div>

                {/* Right: time + badge */}
                <div className="ml-4 flex shrink-0 flex-col items-end gap-2">
                  <span className="text-[10px] font-bold uppercase tracking-wider text-[#737686]">
                    {lead.timeAgo}
                  </span>
                  <span
                    className={cn(
                      "whitespace-nowrap rounded-full px-3 py-1 text-[10px] font-bold",
                      STATUS_STYLES[lead.status]
                    )}
                  >
                    {lead.status}
                  </span>
                </div>

              </Link>
            ))}
          </div>
        </div>
      </section>
    </div>
  );
}
