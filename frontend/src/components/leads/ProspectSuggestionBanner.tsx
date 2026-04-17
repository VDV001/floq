"use client";

import { useEffect, useState } from "react";
import { Link2, X, Mail, Send, CheckCircle2 } from "lucide-react";
import {
  api,
  ProspectSuggestion,
  SuggestionConfidence,
} from "@/lib/api";

interface Props {
  leadId: string;
  /** Called after a successful link/dismiss so the parent can refresh the lead. */
  onChanged?: () => void;
}

const CONFIDENCE_LABEL: Record<SuggestionConfidence, string> = {
  high: "Высокая уверенность",
  medium: "Средняя уверенность",
  low: "Низкая уверенность",
};

const CONFIDENCE_REASON: Record<SuggestionConfidence, string> = {
  high: "совпадает имя и компания",
  medium: "совпадает имя и домен email",
  low: "совпадает только имя",
};

const CONFIDENCE_STYLES: Record<SuggestionConfidence, { pill: string; border: string }> = {
  high: {
    pill: "bg-[#dcf7e3] text-[#0d7a2c] border-[#0d7a2c]/20",
    border: "border-l-4 border-[#0d7a2c]",
  },
  medium: {
    pill: "bg-[#fff3cd] text-[#8a5a00] border-[#8a5a00]/20",
    border: "border-l-4 border-[#e0a800]",
  },
  low: {
    pill: "bg-[#fde2e4] text-[#a00025] border-[#a00025]/20",
    border: "border-l-4 border-[#c82333]",
  },
};

/**
 * ProspectSuggestionBanner surfaces cross-channel prospect matches that the
 * backend could not auto-link (different identifier per channel). The user
 * confirms or rejects each suggestion manually — see issue #6.
 */
export function ProspectSuggestionBanner({ leadId, onChanged }: Props) {
  const [suggestions, setSuggestions] = useState<ProspectSuggestion[] | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    api
      .getProspectSuggestions(leadId)
      .then((items) => {
        if (!cancelled) setSuggestions(items);
      })
      .catch(() => {
        if (!cancelled) setSuggestions([]);
      });
    return () => {
      cancelled = true;
    };
  }, [leadId]);

  if (!suggestions || suggestions.length === 0) {
    return null;
  }

  async function handleLink(prospectId: string) {
    setBusyId(prospectId);
    try {
      await api.linkProspect(leadId, prospectId);
      setSuggestions((prev) => prev?.filter((s) => s.prospect_id !== prospectId) ?? []);
      onChanged?.();
    } catch {
      alert("Не удалось связать — попробуйте ещё раз");
    } finally {
      setBusyId(null);
    }
  }

  async function handleDismiss(prospectId: string) {
    setBusyId(prospectId);
    try {
      await api.dismissProspectSuggestion(leadId, prospectId);
      setSuggestions((prev) => prev?.filter((s) => s.prospect_id !== prospectId) ?? []);
      onChanged?.();
    } catch {
      alert("Не удалось отклонить — попробуйте ещё раз");
    } finally {
      setBusyId(null);
    }
  }

  return (
    <section className="mb-8 rounded-xl border border-[#c3c6d7]/30 bg-white p-5 shadow-sm">
      <header className="mb-4 flex items-center gap-2">
        <Link2 className="size-4 text-[#004ac6]" />
        <h3 className="text-sm font-bold text-[#0d1c2e]">
          Возможно, это тот же человек из проспектов
        </h3>
        <span className="ml-auto rounded-full bg-[#eff4ff] px-2 py-0.5 text-[0.65rem] font-semibold text-[#434655]">
          {suggestions.length}
        </span>
      </header>

      <ul className="space-y-3">
        {suggestions.map((s) => {
          // Defensive fallback: if the backend ever returns an unknown
          // confidence string, treat it as "low" rather than crashing.
          const styles = CONFIDENCE_STYLES[s.confidence] ?? CONFIDENCE_STYLES.low;
          const label = CONFIDENCE_LABEL[s.confidence] ?? CONFIDENCE_LABEL.low;
          const reason = CONFIDENCE_REASON[s.confidence] ?? CONFIDENCE_REASON.low;
          const isBusy = busyId === s.prospect_id;
          return (
            <li
              key={s.prospect_id}
              className={`flex items-start justify-between gap-4 rounded-lg bg-[#f7f9fd] p-4 ${styles.border}`}
            >
              <div className="min-w-0 flex-1">
                <div className="mb-1.5 flex flex-wrap items-center gap-2">
                  <span className="font-semibold text-[#0d1c2e]">{s.name}</span>
                  {s.company && (
                    <span className="text-sm text-[#434655]">· {s.company}</span>
                  )}
                  <span
                    className={`ml-auto rounded-full border px-2 py-0.5 text-[0.65rem] font-semibold ${styles.pill}`}
                    title={reason}
                  >
                    {label}
                  </span>
                </div>
                <p className="mb-2 text-xs text-[#737686]">{reason}</p>
                <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-[#434655]">
                  {s.email && (
                    <span className="inline-flex items-center gap-1">
                      <Mail className="size-3" />
                      {s.email}
                    </span>
                  )}
                  {s.telegram_username && (
                    <span className="inline-flex items-center gap-1">
                      <Send className="size-3" />@{s.telegram_username}
                    </span>
                  )}
                  {s.source_name && (
                    <span className="rounded-full bg-[#eff4ff] px-2 py-0.5 text-[0.65rem] font-medium text-[#004ac6]">
                      {s.source_name}
                    </span>
                  )}
                </div>
              </div>

              <div className="flex shrink-0 flex-col gap-2">
                <button
                  onClick={() => handleLink(s.prospect_id)}
                  disabled={isBusy}
                  className="inline-flex items-center gap-1.5 rounded-lg bg-[#004ac6] px-3 py-1.5 text-xs font-semibold text-white transition-colors hover:bg-[#003a9c] disabled:opacity-50"
                >
                  <CheckCircle2 className="size-3.5" />
                  Связать
                </button>
                <button
                  onClick={() => handleDismiss(s.prospect_id)}
                  disabled={isBusy}
                  className="inline-flex items-center gap-1.5 rounded-lg border border-[#c3c6d7]/40 bg-white px-3 py-1.5 text-xs font-semibold text-[#434655] transition-colors hover:bg-[#eff4ff] disabled:opacity-50"
                >
                  <X className="size-3.5" />
                  Отклонить
                </button>
              </div>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
