"use client";

import { useState, useEffect, useCallback } from "react";
import {
  Search,
  Send,
  Pencil,
  X,
  Clock,
  Bot,
  CheckCheck,
} from "lucide-react";
import { api, type OutboundMessage } from "@/lib/api";
import { Switch } from "@/components/ui/switch";

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

const AVATAR_BGS = [
  "bg-[#d5e0f8]",
  "bg-[#e1e0ff]",
  "bg-[#d8e3fb]",
  "bg-[#dbe1ff]",
  "bg-[#d5f0e8]",
];

const ITEMS_PER_PAGE = 10;

type ChannelFilter = "all" | "email" | "telegram" | "phone_call";
type StatusFilter = "all" | "sent" | "approved" | "rejected";

interface UIMessage {
  id: string;
  name: string;
  initials: string;
  role: string;
  avatarBg: string;
  step: string;
  sequence: string;
  body: string;
  scheduledAt: string;
  channel: "email" | "telegram" | "phone_call";
  status: string;
}

function formatScheduledAt(iso: string): string {
  const d = new Date(iso);
  const now = new Date();
  const isToday =
    d.getDate() === now.getDate() &&
    d.getMonth() === now.getMonth() &&
    d.getFullYear() === now.getFullYear();
  const time = d.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit" });
  return isToday ? `сегодня, ${time}` : d.toLocaleDateString("ru-RU") + ", " + time;
}

function formatTime(d: Date): string {
  return d.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function mapOutboundToUI(msg: OutboundMessage, idx: number): UIMessage {
  const prospectLabel = `Проспект ${msg.prospect_id.slice(0, 6)}`;
  const initials = prospectLabel
    .split(" ")
    .map((w) => w[0])
    .join("")
    .slice(0, 2)
    .toUpperCase();

  return {
    id: msg.id,
    name: prospectLabel,
    initials,
    role: "",
    avatarBg: AVATAR_BGS[idx % AVATAR_BGS.length],
    step: `Шаг ${msg.step_order}`,
    sequence: msg.sequence_id.slice(0, 8),
    body: msg.body,
    scheduledAt: formatScheduledAt(msg.scheduled_at),
    channel: msg.channel,
    status: msg.status,
  };
}

/* ------------------------------------------------------------------ */
/*  Filter pill component                                              */
/* ------------------------------------------------------------------ */

function FilterPill<T extends string>({
  label,
  value,
  current,
  onChange,
}: {
  label: string;
  value: T;
  current: T;
  onChange: (v: T) => void;
}) {
  const active = value === current;
  return (
    <button
      onClick={() => onChange(value)}
      className={`rounded-full px-3 py-1 text-xs font-bold transition-all ${
        active
          ? "bg-[#004ac6] text-white"
          : "border border-[#c3c6d7] text-[#434655] hover:border-[#004ac6] hover:text-[#004ac6]"
      }`}
    >
      {label}
    </button>
  );
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function OutboundPage() {
  const [autopilot, setAutopilot] = useState(false);
  const [messages, setMessages] = useState<UIMessage[]>([]);
  const [sentMessages, setSentMessages] = useState<UIMessage[]>([]);
  const [tab, setTab] = useState<"queue" | "sent">("queue");
  const [loading, setLoading] = useState(true);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editText, setEditText] = useState("");
  const [search, setSearch] = useState("");
  const [stats, setStats] = useState({ draft: 0, approved: 0, sent: 0, opened: 0, replied: 0, bounced: 0 });
  const [channelFilter, setChannelFilter] = useState<ChannelFilter>("all");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [page, setPage] = useState(1);
  const [lastUpdated, setLastUpdated] = useState<Date>(new Date());
  const [approvingAll, setApprovingAll] = useState(false);

  /* Reset page when filters / tab / search change */
  useEffect(() => {
    setPage(1);
  }, [tab, channelFilter, statusFilter, search]);

  /* Fetch data (reusable) */
  const fetchData = useCallback(async (isInitial: boolean) => {
    try {
      const [queue, sent] = await Promise.all([
        api.getOutboundQueue(),
        api.getOutboundSent(),
      ]);
      setMessages(queue.map(mapOutboundToUI));
      setSentMessages(sent.map(mapOutboundToUI));
      setLastUpdated(new Date());
    } catch {
      // silently ignore
    } finally {
      if (isInitial) setLoading(false);
    }
    api.getOutboundStats().then(setStats).catch(() => {});
  }, []);

  /* Initial load */
  useEffect(() => {
    fetchData(true);
  }, [fetchData]);

  /* Auto-poll every 10 seconds */
  useEffect(() => {
    const id = setInterval(() => fetchData(false), 10_000);
    return () => clearInterval(id);
  }, [fetchData]);

  const refreshStats = () => {
    api.getOutboundStats().then(setStats).catch(() => {});
  };

  const handleApprove = async (id: string) => {
    try {
      await api.approveMessage(id);
      setMessages((prev) => prev.filter((m) => m.id !== id));
      refreshStats();
    } catch {
      // silently ignore
    }
  };

  const handleReject = async (id: string) => {
    try {
      await api.rejectMessage(id);
      setMessages((prev) => prev.filter((m) => m.id !== id));
      refreshStats();
    } catch {
      // silently ignore
    }
  };

  const handleEdit = (msg: UIMessage) => {
    setEditingId(msg.id);
    setEditText(msg.body);
  };

  const handleSaveEdit = async (id: string) => {
    try {
      await api.editMessage(id, editText);
      setMessages((prev) =>
        prev.map((m) => (m.id === id ? { ...m, body: editText } : m))
      );
      setEditingId(null);
    } catch {
      // silently ignore
    }
  };

  const handleCancelEdit = () => {
    setEditingId(null);
    setEditText("");
  };

  const handleApproveAll = async () => {
    setApprovingAll(true);
    try {
      for (const msg of messages) {
        await api.approveMessage(msg.id);
      }
      setMessages([]);
      refreshStats();
    } catch {
      // partial success is fine — refetch
      await fetchData(false);
    } finally {
      setApprovingAll(false);
    }
  };

  /* Filtering */
  const activeList = tab === "queue" ? messages : sentMessages;

  const filtered = activeList.filter((m) => {
    // Search filter
    if (search.trim()) {
      const q = search.toLowerCase();
      if (!m.name.toLowerCase().includes(q) && !m.body.toLowerCase().includes(q)) {
        return false;
      }
    }
    // Channel filter
    if (channelFilter !== "all" && m.channel !== channelFilter) return false;
    // Status filter (sent tab only)
    if (tab === "sent" && statusFilter !== "all" && m.status !== statusFilter) return false;
    return true;
  });

  /* Pagination */
  const totalPages = Math.max(1, Math.ceil(filtered.length / ITEMS_PER_PAGE));
  const safePage = Math.min(page, totalPages);
  const paginatedItems = filtered.slice(
    (safePage - 1) * ITEMS_PER_PAGE,
    safePage * ITEMS_PER_PAGE
  );

  return (
    <div className="min-h-full">
      {/* Top search bar */}
      <header className="flex h-16 items-center justify-between px-4 sm:px-6 lg:px-10">
        <div className="flex items-center gap-3">
          <div className="relative w-96">
            <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-slate-400" />
            <input
              type="text"
              placeholder="Поиск по очереди..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full rounded-full border-none bg-[#eff4ff] py-2 pl-10 pr-4 text-sm placeholder-slate-400 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
            />
          </div>
          {tab === "queue" && messages.length > 0 && (
            <button
              onClick={handleApproveAll}
              disabled={approvingAll}
              className="flex items-center gap-1.5 rounded-full border border-[#004ac6] px-4 py-2 text-xs font-bold text-[#004ac6] transition-all hover:bg-[#004ac6] hover:text-white disabled:opacity-50"
            >
              <CheckCheck className="size-3.5" />
              {approvingAll ? "Подтверждение..." : "Подтвердить все"}
            </button>
          )}
        </div>
      </header>

      {/* Page content */}
      <div className="mx-auto max-w-6xl px-4 sm:px-6 lg:px-10 py-8">
        {/* Header */}
        <div className="mb-6">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="mb-1 text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">
                Очередь отправки
              </h2>
              <p className="text-sm font-medium text-[#434655]">
                Контроль качества AI-сообщений перед отправкой
              </p>
            </div>
            <span className="text-[11px] text-[#737686]">
              Обновлено: {formatTime(lastUpdated)}
            </span>
          </div>
        </div>

        {/* Stats — compact horizontal strip */}
        <div className="mb-6 grid grid-cols-3 gap-3 sm:grid-cols-6">
          {[
            { label: "Отправлено", value: stats.sent, color: "text-[#0d1c2e]" },
            { label: "Одобрено", value: stats.approved, color: "text-[#0d1c2e]" },
            { label: "В очереди", value: stats.draft, color: "text-[#004ac6]" },
            { label: "Открыто", value: stats.opened ?? 0, color: "text-green-600" },
            { label: "Ответили", value: stats.replied ?? 0, color: "text-green-600" },
            { label: "Bounce", value: stats.bounced ?? 0, color: "text-red-500" },
          ].map((s) => (
            <div key={s.label} className="rounded-xl bg-white p-4 text-center shadow-sm ring-1 ring-[#c3c6d7]/10">
              <p className={`text-2xl font-black ${s.color}`}>{s.value}</p>
              <p className="mt-1 text-[10px] font-bold uppercase tracking-widest text-[#737686]">{s.label}</p>
            </div>
          ))}
        </div>

        {/* Tabs */}
        <div className="mb-4 flex gap-1 rounded-xl bg-[#eff4ff] p-1">
          <button
            onClick={() => { setTab("queue"); setStatusFilter("all"); }}
            className={`flex-1 rounded-lg px-4 py-2.5 text-sm font-bold transition-all ${
              tab === "queue"
                ? "bg-white text-[#0d1c2e] shadow-sm"
                : "text-[#434655] hover:text-[#0d1c2e]"
            }`}
          >
            В очереди{messages.length > 0 && ` (${messages.length})`}
          </button>
          <button
            onClick={() => setTab("sent")}
            className={`flex-1 rounded-lg px-4 py-2.5 text-sm font-bold transition-all ${
              tab === "sent"
                ? "bg-white text-[#0d1c2e] shadow-sm"
                : "text-[#434655] hover:text-[#0d1c2e]"
            }`}
          >
            Отправленные{sentMessages.length > 0 && ` (${sentMessages.length})`}
          </button>
        </div>

        {/* Filters row */}
        <div className="mb-8 flex flex-wrap items-center gap-2">
          {tab === "queue" ? (
            <>
              <FilterPill label="Все" value="all" current={channelFilter} onChange={setChannelFilter} />
              <FilterPill label="Email" value="email" current={channelFilter} onChange={setChannelFilter} />
              <FilterPill label="Telegram" value="telegram" current={channelFilter} onChange={setChannelFilter} />
              <FilterPill label="Звонок" value="phone_call" current={channelFilter} onChange={setChannelFilter} />
            </>
          ) : (
            <>
              <FilterPill label="Все" value="all" current={statusFilter} onChange={setStatusFilter} />
              <FilterPill label="Отправлено" value="sent" current={statusFilter} onChange={setStatusFilter} />
              <FilterPill label="Одобрено" value="approved" current={statusFilter} onChange={setStatusFilter} />
              <FilterPill label="Отклонено" value="rejected" current={statusFilter} onChange={setStatusFilter} />
            </>
          )}
        </div>

        {/* Autopilot toggle — only in queue tab */}
        {tab === "queue" && <div className="mb-10 flex items-center justify-between rounded-2xl border border-transparent bg-[#eff4ff] p-6 transition-all hover:border-[#c3c6d7]/20 hover:bg-white">
          <div className="flex items-center gap-4">
            <div className="flex size-12 items-center justify-center rounded-full bg-[#e1e0ff] text-[#3e3fcc]">
              <Bot className="size-6" />
            </div>
            <div>
              <h3 className="font-bold text-[#0d1c2e]">Автопилот</h3>
              <p className="text-sm text-[#434655]">
                Сообщения будут отправляться автоматически без вашего одобрения
              </p>
            </div>
          </div>
          <div className="flex items-center gap-4">
            <span className="text-xs font-bold uppercase tracking-wider text-[#737686]">
              {autopilot ? "Вкл" : "Выкл"}
            </span>
            <Switch checked={autopilot} onCheckedChange={setAutopilot} />
          </div>
        </div>}

        {/* Message list */}
        <div className="space-y-4">
          {!loading && paginatedItems.length === 0 && (
            <div className="flex flex-col items-center justify-center rounded-2xl bg-white py-16 text-center">
              <Send className="mb-4 size-10 text-[#c3c6d7]" />
              <p className="text-lg font-semibold text-[#434655]">
                {tab === "queue" ? "Нет сообщений в очереди" : "Нет отправленных сообщений"}
              </p>
              <p className="mt-1 text-sm text-[#737686]">
                {tab === "queue"
                  ? "Новые сообщения появятся здесь после генерации AI"
                  : "Одобренные и отправленные сообщения будут отображаться здесь"}
              </p>
            </div>
          )}
          {paginatedItems.map((msg) => (
            <div
              key={msg.id}
              className="flex flex-col items-start gap-6 rounded-2xl border border-transparent bg-white p-6 transition-all duration-300 hover:border-[#004ac6]/10 hover:shadow-xl hover:shadow-blue-900/5 lg:flex-row lg:items-center"
            >
              {/* Left: prospect info */}
              <div className="flex w-full flex-shrink-0 items-center gap-4 lg:w-64">
                <div
                  className={`flex size-12 shrink-0 items-center justify-center rounded-full text-lg font-bold ${msg.avatarBg} text-[#0d1c2e]`}
                >
                  {msg.initials}
                </div>
                <div className="min-w-0">
                  <h4 className="truncate font-bold text-[#0d1c2e]">
                    {msg.name}
                  </h4>
                  <p className="truncate text-xs text-[#434655]">{msg.role}</p>
                </div>
              </div>

              {/* Center: content */}
              <div className="min-w-0 flex-1">
                <div className="mb-2 flex items-center gap-3">
                  <span className="rounded bg-[#e1e0ff] px-2 py-0.5 text-[10px] font-black uppercase text-[#2f2ebe]">
                    {msg.step}
                  </span>
                  <span className="text-[11px] font-bold uppercase tracking-tight text-[#737686]">
                    Sequence: {msg.sequence}
                  </span>
                  <span className={`rounded-full px-2 py-0.5 text-[10px] font-bold uppercase ${
                    msg.channel === "email"
                      ? "bg-blue-50 text-blue-600"
                      : msg.channel === "telegram"
                        ? "bg-sky-50 text-sky-600"
                        : "bg-amber-50 text-amber-600"
                  }`}>
                    {msg.channel === "email" ? "Email" : msg.channel === "telegram" ? "Telegram" : "Звонок"}
                  </span>
                </div>
                {editingId === msg.id ? (
                  <div className="flex flex-col gap-2">
                    <textarea
                      value={editText}
                      onChange={(e) => setEditText(e.target.value)}
                      rows={4}
                      className="w-full rounded-xl border border-[#c3c6d7] bg-white p-3 text-sm leading-relaxed text-[#434655] outline-none focus:border-[#004ac6] focus:ring-2 focus:ring-[#004ac6]/20"
                    />
                    <div className="flex gap-2">
                      <button
                        onClick={() => handleSaveEdit(msg.id)}
                        className="rounded-lg bg-[#004ac6] px-3 py-1.5 text-xs font-bold text-white hover:opacity-90"
                      >
                        Сохранить
                      </button>
                      <button
                        onClick={handleCancelEdit}
                        className="rounded-lg border border-[#c3c6d7] px-3 py-1.5 text-xs font-bold text-[#434655] hover:bg-[#eff4ff]"
                      >
                        Отмена
                      </button>
                    </div>
                  </div>
                ) : (
                  <p className="line-clamp-2 text-sm italic leading-relaxed text-[#434655]">
                    &ldquo;{msg.body}&rdquo;
                  </p>
                )}
                <div className="mt-2 flex items-center gap-2 text-[11px] font-bold uppercase text-[#004ac6]/60">
                  <Clock className="size-3.5" />
                  Запланировано: {msg.scheduledAt}
                </div>
              </div>

              {/* Right: actions (queue only) or status badge (sent) */}
              {tab === "queue" ? (
                <div className="flex w-full items-center gap-2 lg:w-auto">
                  <button
                    onClick={() => handleApprove(msg.id)}
                    className="flex flex-1 items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-[#004ac6] to-[#2563eb] px-4 py-2.5 text-xs font-bold text-white transition-opacity hover:opacity-90 lg:flex-none"
                  >
                    <Send className="size-3.5" />
                    Подтвердить
                  </button>
                  <button
                    onClick={() => handleEdit(msg)}
                    className="flex size-10 items-center justify-center rounded-xl border border-[#c3c6d7] text-[#434655] transition-colors hover:bg-[#eff4ff]"
                  >
                    <Pencil className="size-[18px]" />
                  </button>
                  <button
                    onClick={() => handleReject(msg.id)}
                    className="flex size-10 items-center justify-center rounded-xl text-[#ba1a1a] transition-colors hover:bg-[#ffdad6]/20"
                  >
                    <X className="size-[18px]" />
                  </button>
                </div>
              ) : (
                <div className="flex shrink-0 items-center gap-2">
                  <span className={`rounded-full px-3 py-1 text-[10px] font-bold uppercase tracking-wider ${
                    msg.status === "sent"
                      ? "bg-green-100 text-green-700"
                      : msg.status === "rejected"
                        ? "bg-red-100 text-red-600"
                        : "bg-blue-100 text-blue-700"
                  }`}>
                    {msg.status === "sent" ? "Отправлено" : msg.status === "rejected" ? "Отклонено" : "Одобрено"}
                  </span>
                </div>
              )}
            </div>
          ))}
        </div>

        {/* Pagination */}
        {filtered.length > ITEMS_PER_PAGE && (
          <div className="mt-8 flex items-center justify-center gap-4">
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={safePage <= 1}
              className="rounded-lg border border-[#c3c6d7] px-4 py-2 text-sm font-bold text-[#434655] transition-colors hover:bg-[#eff4ff] disabled:opacity-40 disabled:hover:bg-transparent"
            >
              &larr; Назад
            </button>
            <span className="text-sm font-medium text-[#737686]">
              {(safePage - 1) * ITEMS_PER_PAGE + 1}–{Math.min(safePage * ITEMS_PER_PAGE, filtered.length)} из {filtered.length}
            </span>
            <button
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={safePage >= totalPages}
              className="rounded-lg border border-[#c3c6d7] px-4 py-2 text-sm font-bold text-[#434655] transition-colors hover:bg-[#eff4ff] disabled:opacity-40 disabled:hover:bg-transparent"
            >
              Далее &rarr;
            </button>
          </div>
        )}

        {/* Footer */}
        <footer className="mt-20 flex flex-col items-center justify-between gap-4 border-t border-[#c3c6d7]/10 py-10 md:flex-row">
          <p className="text-sm font-medium text-[#737686]">
            Создано Floq AI Sales Engine
          </p>
          <div className="flex gap-6 text-xs font-bold uppercase tracking-widest text-[#c3c6d7]">
            <a className="transition-colors hover:text-[#004ac6]" href="#">
              Политика
            </a>
            <a className="transition-colors hover:text-[#004ac6]" href="#">
              Условия
            </a>
            <a className="transition-colors hover:text-[#004ac6]" href="#">
              API
            </a>
          </div>
        </footer>
      </div>
    </div>
  );
}
