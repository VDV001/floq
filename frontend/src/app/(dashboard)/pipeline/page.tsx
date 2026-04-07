"use client";

import { useState, useEffect } from "react";
import { api, Lead } from "@/lib/api";
import { cn } from "@/lib/utils";
import {
  Mail,
  Send,
  TrendingUp,
  Clock,
  DollarSign,
  Sparkles,
  Brain,
  X,
  Target,
  Calendar,
  Zap,
} from "lucide-react";

// ---------- Types ----------

type Channel = "all" | "email" | "telegram";

interface PipelineLead {
  id: string;
  name: string;
  company: string;
  channel: "email" | "telegram";
  preview?: string;
  timeAgo: string;
}

interface PipelineColumn {
  key: string;
  title: string;
  count: number;
  dotColor: string;
  badgeStyle: string;
  leads: PipelineLead[];
}

// ---------- Helpers ----------

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

const STATUS_CONFIG: Record<
  Lead["status"],
  { key: string; title: string; dotColor: string; badgeStyle: string }
> = {
  new: {
    key: "new",
    title: "Новый",
    dotColor: "#004ac6",
    badgeStyle: "bg-blue-50 text-blue-600",
  },
  qualified: {
    key: "qualified",
    title: "Квалифицирован",
    dotColor: "#3e3fcc",
    badgeStyle: "bg-purple-50 text-purple-600",
  },
  in_conversation: {
    key: "in_conversation",
    title: "В диалоге",
    dotColor: "#f59e0b",
    badgeStyle: "border border-amber-300 bg-amber-50 text-amber-700",
  },
  followup: {
    key: "followup",
    title: "Фоллоуап",
    dotColor: "#f97316",
    badgeStyle: "border border-orange-300 bg-orange-50 text-orange-700",
  },
  closed: {
    key: "closed",
    title: "Закрыт",
    dotColor: "#10b981",
    badgeStyle: "border border-green-300 bg-green-50 text-green-700",
  },
};

const COLUMN_ORDER: Lead["status"][] = [
  "new",
  "qualified",
  "in_conversation",
  "followup",
  "closed",
];

const CHANNEL_FILTERS: { label: string; value: Channel }[] = [
  { label: "Все каналы", value: "all" },
  { label: "Email", value: "email" },
  { label: "Telegram", value: "telegram" },
];

// ---------- Small components ----------

function ChannelBadge({ channel }: { channel: "email" | "telegram" }) {
  if (channel === "telegram") {
    return (
      <div className="flex items-center gap-1.5">
        <div className="flex size-5 items-center justify-center rounded-md bg-sky-500/10">
          <Send className="size-3 text-sky-500" />
        </div>
        <span className="text-[11px] font-medium text-[#737686]">
          Telegram
        </span>
      </div>
    );
  }
  return (
    <div className="flex items-center gap-1.5">
      <div className="flex size-5 items-center justify-center rounded-md bg-red-500/10">
        <Mail className="size-3 text-red-500" />
      </div>
      <span className="text-[11px] font-medium text-[#737686]">Email</span>
    </div>
  );
}

// ---------- Lead card + detail modal ----------

function LeadCard({ lead }: { lead: PipelineLead }) {
  const [expanded, setExpanded] = useState(false);
  const [qual, setQual] = useState<{
    identified_need: string;
    estimated_budget: string;
    deadline: string;
    score: number;
    score_reason: string;
    recommended_action: string;
  } | null>(null);

  const openDetail = () => {
    setExpanded(true);
    api.getQualification(lead.id).then(setQual).catch(() => {});
  };

  return (
    <>
      {/* Compact card */}
      <div
        onClick={openDetail}
        className="cursor-pointer rounded-xl border border-[#c3c6d7]/5 bg-white p-3 shadow-sm transition-shadow hover:shadow-md"
      >
        <div className="mb-2 flex items-center justify-between">
          <ChannelBadge channel={lead.channel} />
          <span className="text-[10px] text-[#737686]">{lead.timeAgo}</span>
        </div>
        <p className="text-sm font-bold text-[#0d1c2e]">{lead.name}</p>
        {lead.company && (
          <p className="text-xs text-[#737686]">{lead.company}</p>
        )}
      </div>

      {/* Detail modal */}
      {expanded && (
        <>
          <div
            className="fixed inset-0 z-40 bg-black/20 backdrop-blur-sm"
            onClick={() => setExpanded(false)}
          />
          <div className="fixed inset-y-8 right-8 z-50 w-[min(28rem,calc(100vw-2rem))] overflow-y-auto rounded-2xl bg-white p-6 shadow-2xl">
            {/* Header */}
            <div className="mb-6 flex items-start justify-between">
              <div>
                <ChannelBadge channel={lead.channel} />
                <h3 className="mt-2 text-xl font-extrabold text-[#0d1c2e]">{lead.name}</h3>
                {lead.company && <p className="text-sm text-[#737686]">{lead.company}</p>}
                <p className="mt-1 text-xs text-[#737686]">{lead.timeAgo}</p>
              </div>
              <button onClick={() => setExpanded(false)} className="rounded-lg p-1.5 text-[#434655] hover:bg-[#eff4ff]">
                <X className="size-5" />
              </button>
            </div>

            {/* Qualification */}
            {qual ? (
              <div className="space-y-4">
                {/* Score */}
                <div className="flex items-center gap-3 rounded-xl bg-[#eff4ff] p-4">
                  <div className={`flex size-12 items-center justify-center rounded-full text-lg font-black text-white ${
                    qual.score >= 7 ? "bg-green-500" : qual.score >= 4 ? "bg-amber-500" : "bg-red-500"
                  }`}>
                    {qual.score}
                  </div>
                  <div className="flex-1">
                    <p className="text-xs font-bold uppercase tracking-wider text-[#737686]">Скор квалификации</p>
                    <p className="mt-0.5 text-sm text-[#434655]">{qual.score_reason}</p>
                  </div>
                </div>

                {/* Need */}
                <div className="rounded-xl border border-[#c3c6d7]/10 p-4">
                  <div className="mb-2 flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-[#004ac6]">
                    <Target className="size-3.5" />
                    Потребность
                  </div>
                  <p className="text-sm text-[#434655]">{qual.identified_need}</p>
                </div>

                {/* Budget + Deadline */}
                <div className="grid grid-cols-2 gap-3">
                  <div className="rounded-xl border border-[#c3c6d7]/10 p-4">
                    <div className="mb-1 flex items-center gap-1.5 text-xs font-bold uppercase tracking-wider text-[#737686]">
                      <DollarSign className="size-3.5" />
                      Бюджет
                    </div>
                    <p className="text-sm font-medium text-[#0d1c2e]">{qual.estimated_budget}</p>
                  </div>
                  <div className="rounded-xl border border-[#c3c6d7]/10 p-4">
                    <div className="mb-1 flex items-center gap-1.5 text-xs font-bold uppercase tracking-wider text-[#737686]">
                      <Calendar className="size-3.5" />
                      Сроки
                    </div>
                    <p className="text-sm font-medium text-[#0d1c2e]">{qual.deadline}</p>
                  </div>
                </div>

                {/* Recommended action */}
                <div className="rounded-xl bg-[#e1e0ff]/30 p-4">
                  <div className="mb-2 flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-[#3e3fcc]">
                    <Zap className="size-3.5" />
                    Рекомендация
                  </div>
                  <p className="text-sm text-[#434655]">{qual.recommended_action}</p>
                </div>
              </div>
            ) : (
              <div className="py-8 text-center text-sm text-[#737686]">
                Нет данных квалификации
              </div>
            )}

            {/* Action buttons */}
            <div className="mt-6 flex gap-2">
              <a
                href={`/inbox/${lead.id}`}
                className="flex flex-1 items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-[#004ac6] to-[#2563eb] py-3 text-sm font-bold text-white shadow-md hover:-translate-y-0.5 hover:shadow-lg transition-all"
              >
                Открыть лида
              </a>
            </div>
          </div>
        </>
      )}
    </>
  );
}

// ---------- Kanban column ----------

function KanbanColumn({ column }: { column: PipelineColumn }) {
  return (
    <div className="flex min-w-[280px] shrink-0 flex-col">
      {/* Header */}
      <div className="mb-3 flex items-center gap-2">
        <span
          className="size-2.5 rounded-full"
          style={{ backgroundColor: column.dotColor }}
        />
        <span className="text-sm font-semibold text-[#0d1c2e]">
          {column.title}
        </span>
        <span
          className={cn(
            "rounded-full px-2 py-0.5 text-xs font-medium",
            column.badgeStyle,
          )}
        >
          {column.count}
        </span>
      </div>

      {/* Cards */}
      <div className="flex flex-col gap-3">
        {column.leads.map((lead) => (
          <LeadCard key={lead.id} lead={lead} />
        ))}
      </div>
    </div>
  );
}

// ---------- Metric cards ----------

function MetricCards({
  totalActive,
  columnCounts,
}: {
  totalActive: number;
  columnCounts: Record<string, number>;
}) {
  const newCount = columnCounts["new"] || 0;
  const qualifiedCount = columnCounts["qualified"] || 0;
  const conversionPct =
    newCount > 0 ? Math.round((qualifiedCount / newCount) * 100) : 0;
  const followupCount = columnCounts["followup"] || 0;

  return (
    <div className="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
      {/* Conversion */}
      <div className="rounded-xl border border-[#c3c6d7]/5 bg-white p-4 shadow-sm">
        <div className="mb-2 flex items-center gap-2">
          <div className="flex size-8 items-center justify-center rounded-lg bg-blue-500/10">
            <TrendingUp className="size-4 text-blue-500" />
          </div>
          <span className="text-xs font-medium text-[#737686]">
            Конверсия (New &rarr; Qual)
          </span>
        </div>
        <p className="text-3xl font-bold text-[#0d1c2e]">{conversionPct}%</p>
        <div className="mt-2 h-1.5 w-full rounded-full bg-blue-500/10">
          <div
            className="h-1.5 rounded-full bg-blue-500"
            style={{ width: `${Math.min(conversionPct, 100)}%` }}
          />
        </div>
      </div>

      {/* Total active */}
      <div className="rounded-xl border border-[#c3c6d7]/5 bg-white p-4 shadow-sm">
        <div className="mb-2 flex items-center gap-2">
          <div className="flex size-8 items-center justify-center rounded-lg bg-purple-500/10">
            <Clock className="size-4 text-purple-500" />
          </div>
          <span className="text-xs font-medium text-[#737686]">
            Всего активных
          </span>
        </div>
        <p className="text-3xl font-bold text-[#0d1c2e]">{totalActive}</p>
      </div>

      {/* New leads */}
      <div className="rounded-xl border border-[#c3c6d7]/5 bg-white p-4 shadow-sm">
        <div className="mb-2 flex items-center gap-2">
          <div className="flex size-8 items-center justify-center rounded-lg bg-blue-500/10">
            <DollarSign className="size-4 text-blue-500" />
          </div>
          <span className="text-xs font-medium text-[#737686]">
            Новых лидов
          </span>
        </div>
        <p className="text-3xl font-bold text-[#0d1c2e]">{newCount}</p>
      </div>

      {/* Floq AI insight */}
      <div className="relative overflow-hidden rounded-xl border border-[#d8d7ff] bg-[#e1e0ff] p-4 shadow-sm">
        <div className="mb-2 flex items-center gap-2">
          <div className="flex size-8 items-center justify-center rounded-lg bg-[#3e3fcc]/10">
            <Sparkles className="size-4 text-[#3e3fcc]" />
          </div>
          <span className="text-xs font-semibold text-[#3e3fcc]">
            Floq AI Инсайт
          </span>
        </div>
        <p className="text-sm leading-relaxed text-[#2f2ebe]">
          {followupCount > 0
            ? `${followupCount} сделок в «Фоллоуап» требуют срочного внимания`
            : "Все лиды в работе, фоллоуапов нет"}
        </p>
        {/* Decorative brain icon */}
        <Brain className="absolute -bottom-2 -right-2 size-20 text-[#3e3fcc] opacity-10" />
      </div>
    </div>
  );
}

// ---------- Page ----------

export default function PipelinePage() {
  const [activeChannel, setActiveChannel] = useState<Channel>("all");
  const [leads, setLeads] = useState<Lead[]>([]);
  const [qualifications, setQualifications] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchLeads = () => {
      api.getLeads().then((data) => {
        setLeads(data);
        // Fetch qualifications for all leads
        data.forEach((l) => {
          api.getQualification(l.id).then((q) => {
            if (q?.identified_need) {
              setQualifications((prev) => ({ ...prev, [l.id]: q.identified_need }));
            }
          }).catch(() => {});
        });
      }).catch(() => {}).finally(() => setLoading(false));
    };
    fetchLeads();
    const interval = setInterval(fetchLeads, 5000);
    return () => clearInterval(interval);
  }, []);

  // Group leads by status into columns
  const columnCounts: Record<string, number> = {};
  const columns: PipelineColumn[] = COLUMN_ORDER.map((status) => {
    const config = STATUS_CONFIG[status];
    const statusLeads = leads.filter((l) => l.status === status);
    columnCounts[config.key] = statusLeads.length;
    return {
      ...config,
      count: statusLeads.length,
      leads: statusLeads.map((l) => ({
        id: l.id,
        name: l.contact_name,
        company: l.company || "",
        channel: l.channel,
        preview: qualifications[l.id] || (l.first_message === "/start" ? undefined : l.first_message) || undefined,
        timeAgo: getTimeAgo(l.created_at),
      })),
    };
  });

  const totalActive = leads.filter((l) => l.status !== "closed").length;

  const filteredColumns = columns.map((col) => {
    if (activeChannel === "all") return col;
    return {
      ...col,
      leads: col.leads.filter((l) => l.channel === activeChannel),
    };
  });

  return (
    <div className="flex h-full flex-col bg-[#f8f9ff]">
      {/* Header */}
      <div className="border-b border-[#c3c6d7]/10 bg-white/60 px-6 pb-4 pt-6 backdrop-blur">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <div className="flex items-center gap-3">
              <h1 className="text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">
                Воронка продаж
              </h1>
              {loading && (
                <div className="size-5 animate-spin rounded-full border-2 border-[#3b6ef6] border-t-transparent" />
              )}
            </div>
            <p className="mt-1 text-sm text-[#434655]">
              Управление лидами в реальном времени с поддержкой Floq AI
            </p>
          </div>

          {/* Channel filter */}
          <div className="flex items-center gap-1 rounded-xl bg-[#eff4ff] p-1">
            {CHANNEL_FILTERS.map((f) => (
              <button
                key={f.value}
                onClick={() => setActiveChannel(f.value)}
                className={cn(
                  "rounded-lg px-3.5 py-1.5 text-sm font-medium transition-colors",
                  activeChannel === f.value
                    ? "bg-white text-[#0d1c2e] shadow"
                    : "text-[#434655] hover:text-[#0d1c2e]",
                )}
              >
                {f.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto p-6">
        <MetricCards totalActive={totalActive} columnCounts={columnCounts} />

        {/* Kanban board */}
        <div className="flex gap-6 overflow-x-auto pb-4">
          {filteredColumns.map((col) => (
            <KanbanColumn key={col.key} column={col} />
          ))}
        </div>
      </div>

      {/* Floating AI button */}
    </div>
  );
}
