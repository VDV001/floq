"use client";

import { useState, useEffect } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import {
  ArrowLeft,
  Clock,
  Archive,
  ArrowRightLeft,
  Brain,
  Sparkles,
  CheckCircle2,
  Send,
  RefreshCw,
  Zap,
  User,
} from "lucide-react";
import { Switch } from "@/components/ui/switch";
import { ProspectSuggestionBanner } from "@/components/leads/ProspectSuggestionBanner";
import { api, Lead, Message, Qualification, Draft } from "@/lib/api";

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

function formatTime(dateStr: string): string {
  const d = new Date(dateStr);
  return d.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit" });
}

function formatDateLabel(dateStr: string): string {
  const d = new Date(dateStr);
  const today = new Date();
  const yesterday = new Date(today);
  yesterday.setDate(yesterday.getDate() - 1);

  if (d.toDateString() === today.toDateString()) return "Сегодня";
  if (d.toDateString() === yesterday.toDateString()) return "Вчера";

  return d.toLocaleDateString("ru-RU", {
    day: "numeric",
    month: "long",
  });
}

function groupMessagesByDate(messages: Message[]): Map<string, Message[]> {
  const groups = new Map<string, Message[]>();
  for (const msg of messages) {
    const dateKey = new Date(msg.sent_at).toDateString();
    const existing = groups.get(dateKey) || [];
    existing.push(msg);
    groups.set(dateKey, existing);
  }
  return groups;
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function LeadDetailPage() {
  const params = useParams<{ leadId: string }>();
  const leadId = params.leadId;

  const [lead, setLead] = useState<Lead | null>(null);
  const [qualification, setQualification] = useState<Qualification | null>(
    null
  );
  const [messages, setMessages] = useState<Message[]>([]);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [draftText, setDraftText] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [qualLoading, setQualLoading] = useState(true);
  const [draftLoading, setDraftLoading] = useState(true);
  const [regenerating, setRegenerating] = useState(false);
  const [sending, setSending] = useState(false);

  useEffect(() => {
    if (!leadId) return;

    let cancelled = false;

    async function fetchData() {
      try {
        const leadData = await api.getLead(leadId);
        if (cancelled) return;
        setLead(leadData);
      } catch {
        if (!cancelled) setError(true);
      }

      try {
        const msgs = await api.getMessages(leadId);
        if (!cancelled) setMessages(msgs);
      } catch {
        // no messages
      }

      try {
        const qual = await api.getQualification(leadId);
        if (!cancelled) setQualification(qual);
      } catch {
        // not qualified yet
      }
      if (!cancelled) setQualLoading(false);

      try {
        const d = await api.getDraft(leadId);
        if (!cancelled) {
          setDraft(d);
          setDraftText(d.body);
        }
      } catch {
        // no draft
      }
      if (!cancelled) {
        setDraftLoading(false);
        setLoading(false);
      }
    }

    fetchData();

    // Poll every 5 seconds for new messages and qualification updates
    const interval = setInterval(() => {
      api.getMessages(leadId).then(setMessages).catch(() => {});
      api.getQualification(leadId).then(setQualification).catch(() => {});
    }, 5000);

    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [leadId]);

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="size-8 animate-spin rounded-full border-4 border-[#3b6ef6] border-t-transparent" />
      </div>
    );
  }

  if (error || !lead) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-center">
          <p className="text-2xl font-bold text-[#0d1c2e]">Лид не найден</p>
          <Link
            href="/inbox"
            className="mt-4 inline-flex items-center gap-1.5 text-sm text-[#004ac6] hover:underline"
          >
            <ArrowLeft className="size-4" />
            Назад к лидам
          </Link>
        </div>
      </div>
    );
  }

  const initials = getInitials(lead.contact_name);
  const messageGroups = groupMessagesByDate(messages);

  return (
    <div className="flex h-full overflow-hidden">
      {/* ── Main Content ── */}
      <div className="flex-1 overflow-y-auto px-4 sm:px-6 lg:px-10 py-8">
        {/* Back link */}
        <Link
          href="/inbox"
          className="mb-6 inline-flex items-center gap-1.5 text-sm text-[#434655] transition-colors hover:text-[#004ac6]"
        >
          <ArrowLeft className="size-4" />
          Назад
        </Link>

        {/* 1. Contact Info */}
        <section className="mb-10 flex items-start justify-between">
          <div className="flex items-center gap-6">
            <div className="flex size-20 shrink-0 items-center justify-center rounded-2xl bg-[#dbe1ff] text-2xl font-bold text-[#004ac6] shadow-sm">
              {initials}
            </div>
            <div>
              <div className="mb-1 flex items-center gap-3">
                <h2 className="text-xl sm:text-2xl lg:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">
                  {lead.contact_name}
                </h2>
                {lead.channel === "telegram" && (
                  <span className="flex size-6 items-center justify-center rounded-md bg-[#0088cc] text-white">
                    <Send className="size-3.5" />
                  </span>
                )}
              </div>
              <p className="font-medium text-[#434655]">
                {lead.company ? (
                  <>
                    в{" "}
                    <span className="font-bold text-[#004ac6]">
                      {lead.company}
                    </span>
                  </>
                ) : (
                  "—"
                )}
              </p>
              <div className="mt-3 flex gap-4">
                <span className="flex items-center gap-1.5 rounded-full bg-[#eff4ff] px-3 py-1 text-xs text-[#737686]">
                  <Clock className="size-3.5" />
                  {getTimeAgo(lead.updated_at)} назад
                </span>
              </div>
            </div>
          </div>
          <div className="flex gap-3">
            <button className="rounded-lg border border-[#c3c6d7]/30 bg-white px-4 py-2 text-sm font-semibold text-[#0d1c2e] transition-colors hover:bg-[#eff4ff]">
              <Archive className="mr-1.5 inline size-4" />
              Архив
            </button>
            <button className="rounded-lg border border-[#c3c6d7]/30 bg-white px-4 py-2 text-sm font-semibold text-[#0d1c2e] transition-colors hover:bg-[#eff4ff]">
              <ArrowRightLeft className="mr-1.5 inline size-4" />
              Передать
            </button>
          </div>
        </section>

        {/* 2. Prospect suggestions (cross-channel dedup, issue #6) */}
        <ProspectSuggestionBanner
          leadId={leadId}
          onChanged={() => {
            // After link, lead's source_id may have changed — refetch.
            api.getLead(leadId).then(setLead).catch(() => {});
          }}
        />

        {/* 2. AI Qualification */}
        <section className="mb-10">
          <div className="relative overflow-hidden rounded-xl border border-[#c0c1ff]/20 bg-[#e1e0ff]/30 p-6">
            {/* Background decoration */}
            <div className="absolute right-0 top-0 p-8 opacity-5">
              <Sparkles className="size-24" />
            </div>

            {/* Header */}
            <div className="relative z-10 mb-6 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Brain className="size-5 text-[#3e3fcc]" />
                <h3 className="text-lg font-bold text-[#07006c]">
                  ИИ-квалификация лида
                </h3>
              </div>
              {qualification && (
                <div className="flex items-center gap-3">
                  <span className="text-xs font-bold uppercase tracking-wider text-[#2f2ebe]">
                    Оценка
                  </span>
                  <div className="relative flex size-14 items-center justify-center rounded-full border-4 border-[#585be6]/30">
                    <span className="text-sm font-extrabold text-[#3e3fcc]">
                      {qualification.score}
                    </span>
                  </div>
                </div>
              )}
            </div>

            {qualLoading ? (
              <div className="relative z-10 flex items-center gap-2 text-sm text-[#2f2ebe]">
                <div className="size-4 animate-spin rounded-full border-2 border-[#3e3fcc] border-t-transparent" />
                Загрузка квалификации...
              </div>
            ) : qualification ? (
              <>
                {/* 3-column grid */}
                <div className="relative z-10 grid grid-cols-3 gap-6">
                  <div className="rounded-lg bg-white/60 p-4 backdrop-blur-sm">
                    <p className="mb-1 text-[0.65rem] font-bold uppercase text-[#2f2ebe]">
                      Выявленная потребность
                    </p>
                    <p className="text-sm font-medium leading-relaxed text-[#0d1c2e]">
                      {qualification.identified_need}
                    </p>
                  </div>
                  <div className="rounded-lg bg-white/60 p-4 backdrop-blur-sm">
                    <p className="mb-1 text-[0.65rem] font-bold uppercase text-[#2f2ebe]">
                      Оценка бюджета
                    </p>
                    <p className="text-sm font-medium text-[#0d1c2e]">
                      {qualification.estimated_budget}
                    </p>
                  </div>
                  <div className="rounded-lg bg-white/60 p-4 backdrop-blur-sm">
                    <p className="mb-1 text-[0.65rem] font-bold uppercase text-[#2f2ebe]">
                      Сроки
                    </p>
                    <p className="text-sm font-medium text-[#0d1c2e]">
                      {qualification.deadline}
                    </p>
                  </div>
                </div>

                {/* Recommendation */}
                <div className="relative z-10 mt-6 flex w-fit items-center gap-2 rounded-full bg-[#c0c1ff]/40 px-4 py-2 text-xs font-semibold text-[#3e3fcc]">
                  <CheckCircle2 className="size-4" />
                  {qualification.recommended_action}
                </div>
              </>
            ) : (
              <p className="relative z-10 text-sm italic text-[#2f2ebe]/70">
                Ожидает квалификации ИИ...
              </p>
            )}
          </div>
        </section>

        {/* 3. Conversation Thread */}
        <section className="max-w-4xl">
          {messages.length === 0 ? (
            <p className="text-center text-sm text-[#434655]">
              Нет сообщений
            </p>
          ) : (
            Array.from(messageGroups.entries()).map(
              ([dateKey, groupMsgs]) => (
                <div key={dateKey}>
                  {/* Date separator */}
                  <div className="mb-8 flex items-center gap-4">
                    <div className="h-px flex-1 bg-[#c3c6d7]/20" />
                    <span className="text-[0.7rem] font-bold uppercase tracking-widest text-[#737686]">
                      {formatDateLabel(groupMsgs[0].sent_at)}
                    </span>
                    <div className="h-px flex-1 bg-[#c3c6d7]/20" />
                  </div>

                  <div className="space-y-8">
                    {groupMsgs.map((msg) =>
                      msg.direction === "inbound" ? (
                        <div key={msg.id} className="flex items-start gap-4">
                          <div className="flex size-8 shrink-0 items-center justify-center rounded-full bg-[#dbe1ff] text-xs font-bold text-[#004ac6]">
                            {initials}
                          </div>
                          <div className="max-w-[80%] rounded-2xl rounded-tl-none bg-[#dce9ff] p-4">
                            <p className="text-sm leading-relaxed text-[#0d1c2e]">
                              {msg.body}
                            </p>
                            <span className="mt-2 block text-[0.6rem] text-[#434655]">
                              {formatTime(msg.sent_at)}
                            </span>
                          </div>
                        </div>
                      ) : (
                        <div
                          key={msg.id}
                          className="flex flex-row-reverse items-start gap-4"
                        >
                          <div className="flex size-8 shrink-0 items-center justify-center rounded-full bg-[#004ac6] text-white">
                            <User className="size-4" />
                          </div>
                          <div className="max-w-[80%] rounded-2xl rounded-tr-none bg-[#004ac6] p-4 text-white shadow-sm">
                            <p className="text-sm leading-relaxed">
                              {msg.body}
                            </p>
                            <span className="mt-2 block text-[0.6rem] text-[#dbe1ff] opacity-80">
                              {formatTime(msg.sent_at)}
                            </span>
                          </div>
                        </div>
                      )
                    )}
                  </div>
                </div>
              )
            )
          )}
        </section>
      </div>

      {/* ── Right Sidebar: AI Reply Draft ── */}
      <aside className="flex w-96 shrink-0 flex-col border-l border-[#c3c6d7]/10 bg-white p-6">
        {/* Header */}
        <div className="mb-4 flex items-center justify-between">
          <h4 className="text-sm font-bold text-[#0d1c2e]">
            ИИ-черновик ответа
          </h4>
          <div className="flex items-center gap-1 rounded-full bg-[#e1e0ff] px-2 py-1 text-[0.6rem] font-bold uppercase text-[#3e3fcc]">
            <Zap className="size-3" />
            Умный черновик
          </div>
        </div>

        {/* Draft textarea */}
        <div className="relative mb-4 flex-1">
          <div className="h-full rounded-xl border border-[#c3c6d7]/20 bg-[#eff4ff] p-4">
            {draftLoading ? (
              <div className="flex h-full items-center justify-center">
                <div className="size-5 animate-spin rounded-full border-2 border-[#3e3fcc] border-t-transparent" />
              </div>
            ) : draft ? (
              <textarea
                className="h-full w-full resize-none border-none bg-transparent text-sm leading-relaxed text-[#0d1c2e] outline-none"
                value={draftText}
                onChange={(e) => setDraftText(e.target.value)}
                spellCheck={false}
              />
            ) : (
              <p className="text-sm italic text-[#434655]">
                Черновик не создан
              </p>
            )}
          </div>
        </div>

        {/* Actions */}
        <div className="space-y-3">
          <button
            onClick={async () => {
              setRegenerating(true);
              try {
                const d = await api.regenerateDraft(leadId);
                setDraft(d);
                setDraftText(d.body);
              } catch { alert("Ошибка генерации черновика"); }
              finally { setRegenerating(false); }
            }}
            disabled={regenerating}
            className="flex w-full items-center justify-center gap-2 rounded-xl border border-[#c3c6d7]/30 py-3 text-sm font-bold text-[#0d1c2e] transition-all hover:bg-[#eff4ff] disabled:opacity-50"
          >
            {regenerating && <RefreshCw className="size-4 animate-spin" />}
            {regenerating ? "Генерация..." : "Перегенерировать"}
          </button>
          <button
            onClick={async () => {
              if (!draftText.trim()) return;
              setSending(true);
              try {
                await api.sendMessage(leadId, draftText);
                const msgs = await api.getMessages(leadId);
                setMessages(msgs);
                setDraftText("");
                setDraft(null);
              } catch { alert("Ошибка отправки"); }
              finally { setSending(false); }
            }}
            disabled={!draftText.trim() || sending}
            className="flex w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-[#004ac6] to-[#2563eb] py-4 text-sm font-bold text-white shadow-lg shadow-[#004ac6]/20 transition-all hover:opacity-90 active:scale-95 disabled:opacity-50"
          >
            {sending ? <RefreshCw className="size-4 animate-spin" /> : <Send className="size-4" />}
            Отправить ответ
          </button>
        </div>

        {/* Automation Settings */}
        <div className="mt-8 border-t border-[#c3c6d7]/10 pt-8">
          <p className="mb-4 text-[0.65rem] font-bold uppercase text-[#434655]">
            Настройки автоматизации
          </p>
          <div className="flex items-center justify-between py-2">
            <span className="text-xs font-medium text-[#0d1c2e]">
              Авто-фоллоуапы
            </span>
            <Switch defaultChecked />
          </div>
          <div className="flex items-center justify-between py-2">
            <span className="text-xs font-medium text-[#0d1c2e]">
              Согласование черновиков
            </span>
            <Switch defaultChecked />
          </div>
        </div>
      </aside>
    </div>
  );
}
