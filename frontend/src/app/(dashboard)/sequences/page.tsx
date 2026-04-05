"use client";

import { useState, useEffect, useCallback } from "react";
import { api, type Sequence, type SequenceStep } from "@/lib/api";
import {
  Plus,
  Layers,
  Users,
  TrendingUp,
  Sparkles,
  Copy,
  Trash2,
} from "lucide-react";
import { Switch } from "@/components/ui/switch";
import { Separator } from "@/components/ui/separator";

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

const CHANNEL_LABELS: Record<string, { label: string; color: string; bg: string }> = {
  email: { label: "Email", color: "text-blue-600", bg: "bg-blue-50" },
  telegram: { label: "Telegram", color: "text-purple-600", bg: "bg-purple-50" },
  phone_call: { label: "Звонок", color: "text-orange-600", bg: "bg-orange-50" },
};

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function SequencesPage() {
  const [loading, setLoading] = useState(true);
  const [sequences, setSequences] = useState<Sequence[]>([]);
  const [selectedSeqId, setSelectedSeqId] = useState<string | null>(null);
  const [steps, setSteps] = useState<SequenceStep[]>([]);
  const [stepsLoading, setStepsLoading] = useState(false);

  // Fetch all sequences on mount
  useEffect(() => {
    setLoading(true);
    api
      .getSequences()
      .then((data) => {
        setSequences(data);
        // Auto-select first sequence
        if (data.length > 0) {
          setSelectedSeqId(data[0].id);
        }
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  // Fetch steps when a sequence is selected
  useEffect(() => {
    if (!selectedSeqId) {
      setSteps([]);
      return;
    }
    setStepsLoading(true);
    api
      .getSequence(selectedSeqId)
      .then((data) => {
        setSteps(data.steps ?? []);
      })
      .catch(() => setSteps([]))
      .finally(() => setStepsLoading(false));
  }, [selectedSeqId]);

  const handleCreateSequence = useCallback(async () => {
    const name = window.prompt("Название новой секвенции:");
    if (!name || !name.trim()) return;
    try {
      const newSeq = await api.createSequence(name.trim());
      setSequences((prev) => [...prev, newSeq]);
      setSelectedSeqId(newSeq.id);
    } catch {
      // silently ignore
    }
  }, []);

  const handleAddStep = useCallback(async () => {
    if (!selectedSeqId) return;
    const delay = window.prompt("Задержка в днях:", "1");
    if (!delay) return;
    const hint = window.prompt("Подсказка для AI (prompt_hint):", "фоллоуап");
    if (hint === null) return;
    try {
      await api.addStep(selectedSeqId, {
        step_order: steps.length + 1,
        delay_days: parseInt(delay),
        prompt_hint: hint || "фоллоуап",
        channel: "email",
      });
      const data = await api.getSequence(selectedSeqId);
      setSteps(data.steps ?? []);
    } catch {
      // silently ignore
    }
  }, [selectedSeqId, steps]);

  const handleToggleSequence = useCallback(
    async (seqId: string, isActive: boolean) => {
      try {
        await api.toggleSequence(seqId, isActive);
        setSequences((prev) =>
          prev.map((s) => (s.id === seqId ? { ...s, is_active: isActive } : s))
        );
      } catch {
        // silently ignore
      }
    },
    []
  );

  const selectedSequence = sequences.find((s) => s.id === selectedSeqId) ?? null;

  return (
    <div className="min-h-screen bg-[#f8f9ff]">
      {/* ── Header ── */}
      <header className="px-8 pt-8 pb-6">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-2xl sm:text-3xl lg:text-4xl font-extrabold tracking-tight text-[#0d1c2e]">
              Секвенции
            </h1>
            <p className="mt-2 text-[#434655]">
              Автоматические цепочки холодных писем для вашего B2B отдела
            </p>
          </div>
          <button
            onClick={handleCreateSequence}
            className="flex items-center gap-2 rounded-xl bg-gradient-to-r from-[#004ac6] to-[#2563eb] px-5 py-2.5 text-sm font-semibold text-white shadow-lg shadow-[#004ac6]/25 transition hover:shadow-xl hover:shadow-[#004ac6]/30"
          >
            <Plus className="size-4" />
            Новая секвенция
          </button>
        </div>
        {loading && (
          <div className="mt-3 size-5 animate-spin rounded-full border-2 border-[#004ac6] border-t-transparent" />
        )}
      </header>

      {/* ── Bento Grid ── */}
      <div className="grid grid-cols-12 gap-6 px-8 pb-8">
        {/* ════════════════════════════════════════════
            LEFT COLUMN (col-span-4): Campaigns + AI tip
           ════════════════════════════════════════════ */}
        <div className="col-span-4 flex flex-col gap-5">
          <h2 className="flex items-center gap-2 text-sm font-semibold uppercase tracking-wider text-[#434655]">
            <Layers className="size-4" />
            Ваши кампании
          </h2>

          {!loading && sequences.length === 0 && (
            <div className="rounded-2xl border border-dashed border-slate-300 bg-white p-8 text-center">
              <Layers className="mx-auto mb-3 size-8 text-[#c3c6d7]" />
              <p className="text-sm font-medium text-[#434655]">
                Нет секвенций
              </p>
              <p className="mt-1 text-xs text-[#737686]">
                Создайте первую секвенцию, нажав кнопку выше
              </p>
            </div>
          )}

          {sequences.map((seq) => {
            const isSelected = seq.id === selectedSeqId;
            return (
              <div
                key={seq.id}
                onClick={() => setSelectedSeqId(seq.id)}
                className={`cursor-pointer rounded-2xl border bg-white p-5 shadow-sm transition ${
                  isSelected
                    ? "border-l-4 border-l-[#004ac6] border-white/80"
                    : "border-white/80 hover:border-[#004ac6]/20"
                } ${!seq.is_active ? "opacity-70 grayscale" : ""}`}
              >
                <div className="flex items-start justify-between gap-2">
                  <h3 className="text-sm font-semibold text-[#0d1c2e]">
                    {seq.name}
                  </h3>
                  <span
                    className={`shrink-0 rounded-full px-2.5 py-0.5 text-xs font-medium ${
                      seq.is_active
                        ? "bg-green-100 text-green-700"
                        : "bg-slate-100 text-slate-500"
                    }`}
                  >
                    {seq.is_active ? "Активна" : "Пауза"}
                  </span>
                </div>

                <div className="mt-3 flex items-center gap-2 text-xs text-[#737686]">
                  <span>
                    Создана:{" "}
                    {new Date(seq.created_at).toLocaleDateString("ru-RU")}
                  </span>
                </div>

                <Separator className="my-3" />

                <div className="flex items-center justify-between">
                  <Switch
                    checked={seq.is_active}
                    onCheckedChange={(checked) =>
                      handleToggleSequence(seq.id, checked)
                    }
                    size="sm"
                  />
                  <button className="text-xs font-medium text-[#004ac6] hover:underline">
                    Редактировать
                  </button>
                </div>
              </div>
            );
          })}

          {/* AI Tip Card */}
          <div className="rounded-2xl border border-[#3e3fcc]/20 bg-[#e1e0ff]/20 p-5">
            <div className="mb-2 flex items-center gap-2">
              <Sparkles className="size-4 text-[#3e3fcc]" />
              <span className="text-xs font-bold text-[#3e3fcc]">
                AI Совет
              </span>
            </div>
            <p className="text-xs leading-relaxed text-[#0d1c2e]/80">
              {sequences.length === 0
                ? "Создайте первую секвенцию для автоматизации холодного outreach."
                : `У вас ${sequences.length} секвенций. Добавьте шаги с разными каналами для повышения конверсии.`}
            </p>
            <button className="mt-3 text-xs font-semibold text-[#2f2ebe] hover:underline">
              Оптимизировать сейчас &rarr;
            </button>
          </div>
        </div>

        {/* ════════════════════════════════════════════
            MIDDLE COLUMN (col-span-5): Step Timeline
           ════════════════════════════════════════════ */}
        <div className="col-span-5">
          <div className="rounded-2xl bg-white p-6 shadow-sm">
            <h2 className="mb-6 text-base font-semibold text-[#0d1c2e]">
              Шаги секвенции
              {selectedSequence && (
                <span className="ml-2 text-sm font-normal text-[#737686]">
                  — {selectedSequence.name}
                </span>
              )}
            </h2>

            {stepsLoading && (
              <div className="flex justify-center py-8">
                <div className="size-5 animate-spin rounded-full border-2 border-[#004ac6] border-t-transparent" />
              </div>
            )}

            {!stepsLoading && !selectedSeqId && (
              <div className="py-12 text-center">
                <Layers className="mx-auto mb-3 size-8 text-[#c3c6d7]" />
                <p className="text-sm text-[#737686]">
                  Выберите секвенцию слева
                </p>
              </div>
            )}

            {!stepsLoading && selectedSeqId && steps.length === 0 && (
              <div className="py-12 text-center">
                <Plus className="mx-auto mb-3 size-8 text-[#c3c6d7]" />
                <p className="text-sm text-[#737686]">
                  Нет шагов в этой секвенции
                </p>
                <p className="mt-1 text-xs text-[#737686]">
                  Добавьте первый шаг, чтобы начать
                </p>
              </div>
            )}

            {/* Timeline */}
            {!stepsLoading && steps.length > 0 && (
              <div className="relative ml-3">
                {/* Vertical line */}
                <div className="absolute left-[7px] top-0 bottom-0 w-0.5 bg-slate-200" />

                {steps
                  .sort((a, b) => a.step_order - b.step_order)
                  .map((step, idx) => {
                    const isFirst = idx === 0;
                    const isLast = idx === steps.length - 1;
                    const ch = CHANNEL_LABELS[step.channel] ?? CHANNEL_LABELS.email;
                    // Calculate cumulative day
                    const dayNum = steps
                      .slice(0, idx + 1)
                      .reduce((sum, s) => sum + s.delay_days, 0);

                    return (
                      <div
                        key={step.id}
                        className={`relative pl-8 ${isLast ? "" : "mb-8"}`}
                      >
                        <div
                          className={`absolute left-0 top-1 size-4 rounded-full border-2 ${
                            isFirst
                              ? "border-[#004ac6] bg-[#004ac6]"
                              : "border-[#dce9ff] bg-[#dce9ff] transition hover:border-[#004ac6] hover:bg-[#004ac6]"
                          }`}
                        />

                        <div className="flex items-start justify-between">
                          <div>
                            <div className="flex items-center gap-2">
                              <span className="text-xs font-semibold text-[#004ac6]">
                                Шаг {step.step_order}
                                {isFirst && " \u2022 Отправка сразу"}
                              </span>
                              {!isFirst && (
                                <span className="text-xs text-[#737686]">
                                  Задержка: {step.delay_days}{" "}
                                  {step.delay_days === 1 ? "день" : "дней"}
                                </span>
                              )}
                            </div>
                            <p className="mt-1 text-sm font-medium text-[#0d1c2e]">
                              День {dayNum}
                            </p>
                          </div>
                          <div className="flex items-center gap-3">
                            <span
                              className={`rounded-full ${ch.bg} px-2.5 py-0.5 text-xs font-medium ${ch.color}`}
                            >
                              {ch.label}
                            </span>
                            <button className="text-[#737686] hover:text-[#0d1c2e]">
                              <Copy className="size-3.5" />
                            </button>
                            <button className="text-[#737686] hover:text-red-500">
                              <Trash2 className="size-3.5" />
                            </button>
                          </div>
                        </div>

                        {step.prompt_hint && (
                          <p className="mt-2 text-xs leading-relaxed text-[#434655]">
                            {step.prompt_hint}
                          </p>
                        )}

                        <div className="mt-3">
                          <button className="rounded-lg bg-[#004ac6] px-3 py-1.5 text-xs font-medium text-white transition hover:bg-[#004ac6]/90">
                            Сгенерировать пример
                          </button>
                        </div>
                      </div>
                    );
                  })}
              </div>
            )}

            {/* Add step */}
            {selectedSeqId && !stepsLoading && (
              <div className="mt-6 ml-3 pl-8">
                <button onClick={handleAddStep} className="flex w-full items-center justify-center gap-2 rounded-xl border-2 border-dashed border-slate-200 py-3 text-sm font-medium text-[#434655] transition hover:border-[#004ac6] hover:text-[#004ac6]">
                  <Plus className="size-4" />
                  Добавить шаг
                </button>
              </div>
            )}
          </div>
        </div>

        {/* ════════════════════════════════════════════
            RIGHT COLUMN (col-span-3): Prospects + Stats
           ════════════════════════════════════════════ */}
        <div className="col-span-3 flex flex-col gap-5">
          {/* Prospects */}
          <div className="rounded-2xl bg-[#eff4ff]/50 p-5">
            <h2 className="mb-4 flex items-center gap-2 text-sm font-semibold text-[#0d1c2e]">
              <Users className="size-4" />
              Проспекты
            </h2>

            <div className="flex flex-col gap-4">
              <p className="py-6 text-center text-xs text-[#737686]">
                Выберите секвенцию, чтобы увидеть проспекты
              </p>
            </div>

            <div className="mt-4 text-center">
              <button className="text-xs font-medium text-[#004ac6] hover:underline">
                Управление проспектами
              </button>
            </div>
          </div>

          {/* Stats Card */}
          <div className="rounded-2xl bg-[#004ac6] p-5 text-white shadow-lg">
            <p className="text-xs font-medium uppercase tracking-wider text-white/70">
              Эффективность
            </p>
            <div className="mt-2 flex items-baseline gap-2">
              <span className="text-3xl font-bold">—</span>
              <TrendingUp className="size-5 text-green-300" />
            </div>
            <p className="mt-2 text-xs leading-relaxed text-white/70">
              Средний показатель открытий за текущую секвенцию
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
