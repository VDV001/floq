"use client";

import { useEffect, useState } from "react";
import { ShieldCheck, X, AlertCircle, Pencil } from "lucide-react";
import { api, PendingReply } from "@/lib/api";

interface Props {
  leadId: string;
  /** Called after Approve so the parent can refresh messages (the
   *  approved body lands in the conversation thread). */
  onApproved?: () => void;
}

/**
 * PendingReplySection surfaces auto-drafted replies parked by the
 * inbox HITL gate. Today the only source is the Telegram bot's
 * booking-link branch, which used to fire bot.Send the moment
 * DetectCallAgreement matched a phrase; now it enqueues a draft here
 * for the operator to approve before it reaches the customer.
 *
 * Only pending rows render — approved/sent/rejected drafts are
 * already either visible in the conversation thread (sent) or
 * terminated (rejected), so re-displaying them adds noise without
 * value.
 *
 * Each row exposes Approve / Reject / Edit. Edit (#48) flips the row
 * into a textarea + Save/Cancel; Save calls PATCH and refetches so
 * the row reflects whatever the server stored.
 */
export function PendingReplySection({ leadId, onApproved }: Props) {
  const [replies, setReplies] = useState<PendingReply[] | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [draft, setDraft] = useState("");

  useEffect(() => {
    let cancelled = false;
    api
      .getPendingReplies(leadId)
      .then((items) => {
        if (!cancelled) setReplies(items);
      })
      .catch(() => {
        if (!cancelled) setReplies([]);
      });
    return () => {
      cancelled = true;
    };
  }, [leadId]);

  if (!replies) return null;
  const pending = replies.filter((r) => r.status === "pending");
  if (pending.length === 0) return null;

  async function refreshFromServer() {
    try {
      const fresh = await api.getPendingReplies(leadId);
      setReplies(fresh);
    } catch {
      // Best-effort; if the refetch itself fails, leave the optimistic
      // state in place so the operator still sees the action took
      // effect locally. The next page-level refresh will catch up.
    }
  }

  async function handleApprove(id: string) {
    setBusyId(id);
    setError(null);
    try {
      await api.approvePendingReply(id);
      await refreshFromServer();
      onApproved?.();
    } catch {
      setError("Не удалось одобрить — попробуйте ещё раз");
    } finally {
      setBusyId(null);
    }
  }

  async function handleReject(id: string) {
    setBusyId(id);
    setError(null);
    try {
      await api.rejectPendingReply(id);
      await refreshFromServer();
    } catch {
      setError("Не удалось отклонить — попробуйте ещё раз");
    } finally {
      setBusyId(null);
    }
  }

  function startEdit(r: PendingReply) {
    setEditingId(r.id);
    setDraft(r.body);
    setError(null);
  }

  function cancelEdit() {
    setEditingId(null);
    setDraft("");
  }

  async function handleSave(id: string) {
    const trimmed = draft.trim();
    if (trimmed === "") return; // Save button is also disabled — belt + suspenders.
    setBusyId(id);
    setError(null);
    try {
      await api.updatePendingReply(id, trimmed);
      await refreshFromServer();
      setEditingId(null);
      setDraft("");
    } catch {
      setError("Не удалось сохранить — попробуйте ещё раз");
      // Stay in edit mode so the operator can retry without re-typing.
    } finally {
      setBusyId(null);
    }
  }

  return (
    <section
      className="mb-8 rounded-xl border border-[#f5b73c]/40 bg-[#fff8e1] p-5 shadow-sm"
      aria-label="Сообщения, ожидающие подтверждения"
    >
      <header className="mb-4 flex items-center gap-2">
        <ShieldCheck className="size-4 text-[#a06b00]" />
        <h3 className="text-sm font-bold text-[#0d1c2e]">
          Ожидают подтверждения
        </h3>
        <span className="ml-auto rounded-full bg-white px-2 py-0.5 text-[0.65rem] font-semibold text-[#434655]">
          {pending.length}
        </span>
      </header>

      {error && (
        <div
          role="alert"
          className="mb-3 flex items-center gap-2 rounded-lg bg-[#fde2e4] px-3 py-2 text-xs text-[#a00025]"
        >
          <AlertCircle className="size-3.5" />
          {error}
        </div>
      )}

      <ul className="space-y-3">
        {pending.map((r) => {
          const isBusy = busyId === r.id;
          const isEditing = editingId === r.id;
          return (
            <li
              key={r.id}
              className="rounded-lg border border-[#f5b73c]/30 bg-white p-4"
            >
              {isEditing ? (
                <textarea
                  value={draft}
                  onChange={(e) => setDraft(e.target.value)}
                  disabled={isBusy}
                  rows={Math.max(3, draft.split("\n").length)}
                  className="mb-3 w-full resize-y rounded-md border border-[#c3c6d7]/50 bg-white px-3 py-2 text-sm text-[#0d1c2e] focus:border-[#0d7a2c] focus:outline-none disabled:opacity-50"
                  aria-label="Тело сообщения"
                />
              ) : (
                <p className="mb-3 whitespace-pre-wrap text-sm text-[#0d1c2e]">
                  {r.body}
                </p>
              )}
              <div className="flex items-center justify-between gap-3">
                <span className="text-[0.65rem] font-medium uppercase tracking-wide text-[#737686]">
                  {kindLabel(r.kind)} · {channelLabel(r.channel)}
                </span>
                {isEditing ? (
                  <div className="flex gap-2">
                    <button
                      type="button"
                      onClick={cancelEdit}
                      disabled={isBusy}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-[#c3c6d7]/50 bg-white px-3 py-1.5 text-xs font-semibold text-[#434655] transition-colors hover:bg-[#f7f9fd] disabled:opacity-50"
                    >
                      Отмена
                    </button>
                    <button
                      type="button"
                      onClick={() => handleSave(r.id)}
                      disabled={isBusy || draft.trim() === ""}
                      className="inline-flex items-center gap-1.5 rounded-lg bg-[#0d7a2c] px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-[#0a6324] disabled:opacity-50"
                    >
                      Сохранить
                    </button>
                  </div>
                ) : (
                  <div className="flex gap-2">
                    <button
                      type="button"
                      onClick={() => handleReject(r.id)}
                      disabled={isBusy}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-[#c3c6d7]/50 bg-white px-3 py-1.5 text-xs font-semibold text-[#434655] transition-colors hover:bg-[#f7f9fd] disabled:opacity-50"
                    >
                      <X className="size-3.5" />
                      Отклонить
                    </button>
                    <button
                      type="button"
                      onClick={() => startEdit(r)}
                      disabled={isBusy}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-[#c3c6d7]/50 bg-white px-3 py-1.5 text-xs font-semibold text-[#434655] transition-colors hover:bg-[#f7f9fd] disabled:opacity-50"
                    >
                      <Pencil className="size-3.5" />
                      Изменить
                    </button>
                    <button
                      type="button"
                      onClick={() => handleApprove(r.id)}
                      disabled={isBusy}
                      className="inline-flex items-center gap-1.5 rounded-lg bg-[#0d7a2c] px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-[#0a6324] disabled:opacity-50"
                    >
                      <ShieldCheck className="size-3.5" />
                      Одобрить и отправить
                    </button>
                  </div>
                )}
              </div>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

function kindLabel(kind: PendingReply["kind"]): string {
  switch (kind) {
    case "booking_link":
      return "Ссылка на встречу";
    default:
      return kind;
  }
}

function channelLabel(channel: PendingReply["channel"]): string {
  switch (channel) {
    case "telegram":
      return "Telegram";
    case "email":
      return "Email";
    default:
      return channel;
  }
}
