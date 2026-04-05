"use client";

import { useState, useEffect } from "react";
import {
  Search,
  Send,
  Pencil,
  X,
  Clock,
  Bot,
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

function mapOutboundToUI(msg: OutboundMessage, idx: number): UIMessage {
  // Extract prospect name from the message body's first word/greeting if possible,
  // otherwise use a generic label based on prospect_id
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

  useEffect(() => {
    Promise.all([
      api.getOutboundQueue(),
      api.getOutboundSent(),
    ])
      .then(([queue, sent]) => {
        setMessages(queue.map(mapOutboundToUI));
        setSentMessages(sent.map(mapOutboundToUI));
      })
      .catch(() => {})
      .finally(() => setLoading(false));
    api.getOutboundStats().then(setStats).catch(() => {});
  }, []);

  const refreshStats = () => {
    api.getOutboundStats().then(setStats).catch(() => {});
  };

  const handleApprove = async (id: string) => {
    try {
      await api.approveMessage(id);
      setMessages((prev) => prev.filter((m) => m.id !== id));
      refreshStats();
    } catch {
      // silently ignore for now
    }
  };

  const handleReject = async (id: string) => {
    try {
      await api.rejectMessage(id);
      setMessages((prev) => prev.filter((m) => m.id !== id));
      refreshStats();
    } catch {
      // silently ignore for now
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
      // silently ignore for now
    }
  };

  const handleCancelEdit = () => {
    setEditingId(null);
    setEditText("");
  };

  const activeList = tab === "queue" ? messages : sentMessages;
  const filtered = search.trim()
    ? activeList.filter(
        (m) =>
          m.name.toLowerCase().includes(search.toLowerCase()) ||
          m.body.toLowerCase().includes(search.toLowerCase())
      )
    : activeList;

  return (
    <div className="min-h-full">
      {/* Top search bar */}
      <header className="flex h-16 items-center justify-between px-4 sm:px-6 lg:px-10">
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
      </header>

      {/* Page content */}
      <div className="mx-auto max-w-6xl px-4 sm:px-6 lg:px-10 py-8">
        {/* Header */}
        <div className="mb-12 flex flex-col items-start justify-between gap-6 md:flex-row md:items-end">
          <div>
            <h2 className="mb-2 text-2xl sm:text-3xl lg:text-4xl font-extrabold tracking-tight text-[#0d1c2e]">
              Очередь отправки
            </h2>
            <p className="text-sm font-medium text-[#434655]">
              Контроль качества AI-сообщений перед отправкой
              {filtered.length > 0 && (
                <span className="ml-2 rounded-full bg-[#dbe1ff] px-3 py-1 text-xs font-bold text-[#003ea8]">
                  {filtered.length} в очереди
                </span>
              )}
            </p>
          </div>

          {/* Stats */}
          <div className="flex flex-wrap items-center gap-8 rounded-2xl border border-[#c3c6d7]/10 bg-white p-6 shadow-sm">
            <div className="flex flex-col">
              <span className="mb-1 text-[10px] font-bold uppercase tracking-widest text-[#737686]">
                Отправлено
              </span>
              <span className="text-2xl font-black text-[#0d1c2e]">{stats.sent}</span>
            </div>
            <div className="h-10 w-px bg-[#c3c6d7]/20" />
            <div className="flex flex-col">
              <span className="mb-1 text-[10px] font-bold uppercase tracking-widest text-[#737686]">
                Подтверждено
              </span>
              <span className="text-2xl font-black text-[#0d1c2e]">{stats.approved}</span>
            </div>
            <div className="h-10 w-px bg-[#c3c6d7]/20" />
            <div className="flex flex-col">
              <span className="mb-1 text-[10px] font-bold uppercase tracking-widest text-[#737686]">
                В очереди
              </span>
              <span className="text-2xl font-black text-[#004ac6]">{stats.draft}</span>
            </div>
            <div className="h-10 w-px bg-[#c3c6d7]/20" />
            <div className="flex flex-col">
              <span className="mb-1 text-[10px] font-bold uppercase tracking-widest text-[#737686]">
                Открыто
              </span>
              <span className="text-2xl font-black text-green-600">{stats.opened ?? 0}</span>
            </div>
            <div className="h-10 w-px bg-[#c3c6d7]/20" />
            <div className="flex flex-col">
              <span className="mb-1 text-[10px] font-bold uppercase tracking-widest text-[#737686]">
                Ответили
              </span>
              <span className="text-2xl font-black text-green-600">{stats.replied ?? 0}</span>
            </div>
            <div className="h-10 w-px bg-[#c3c6d7]/20" />
            <div className="flex flex-col">
              <span className="mb-1 text-[10px] font-bold uppercase tracking-widest text-[#737686]">
                Bounce
              </span>
              <span className="text-2xl font-black text-red-500">{stats.bounced ?? 0}</span>
            </div>
          </div>
        </div>

        {/* Tabs */}
        <div className="mb-8 flex gap-1 rounded-xl bg-[#eff4ff] p-1">
          <button
            onClick={() => setTab("queue")}
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
          {!loading && filtered.length === 0 && (
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
          {filtered.map((msg) => (
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
