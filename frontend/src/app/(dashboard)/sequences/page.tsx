"use client";

import { useState, useEffect, useCallback } from "react";
import { api, type Sequence, type SequenceStep, type Prospect } from "@/lib/api";
import {
  Plus,
  Layers,
  Users,
  TrendingUp,
  Sparkles,
  Copy,
  Trash2,
  Mail,
  MessageCircle,
  Phone,
  Clock,
  Send,
  X,
  ChevronDown,
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
  const [prospects, setProspects] = useState<Prospect[]>([]);
  const [selectedProspects, setSelectedProspects] = useState<Set<string>>(new Set());
  const [launching, setLaunching] = useState(false);
  const [launchResult, setLaunchResult] = useState<string | null>(null);

  // New sequence inline form
  const [showNewSeq, setShowNewSeq] = useState(false);
  const [newSeqName, setNewSeqName] = useState("");

  // Add step inline form
  const [showAddStep, setShowAddStep] = useState(false);
  const [addStepChannel, setAddStepChannel] = useState<"email" | "telegram">("email");
  const [addStepDelay, setAddStepDelay] = useState(0);
  const [addStepHint, setAddStepHint] = useState("первое касание");

  // Launch options
  const [showLaunchOptions, setShowLaunchOptions] = useState(false);
  const [sendNow, setSendNow] = useState(true);

  // Fetch all sequences on mount
  useEffect(() => {
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
    api.getProspects().then(setProspects).catch(() => {});
  }, []);

  const toggleProspect = (id: string) => {
    setSelectedProspects((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const selectAllProspects = () => {
    if (selectedProspects.size === prospects.length) {
      setSelectedProspects(new Set());
    } else {
      setSelectedProspects(new Set(prospects.map((p) => p.id)));
    }
  };

  const handleLaunch = async () => {
    if (!selectedSeqId || selectedProspects.size === 0) return;
    setLaunching(true);
    setLaunchResult(null);
    setShowLaunchOptions(false);
    try {
      await api.launchSequence(selectedSeqId, Array.from(selectedProspects), sendNow);
      setLaunchResult(`Запущено для ${selectedProspects.size} проспектов`);
      setSelectedProspects(new Set());
      // Refresh prospects to update statuses
      api.getProspects().then(setProspects).catch(() => {});
    } catch {
      setLaunchResult("Ошибка запуска");
    } finally {
      setLaunching(false);
      setTimeout(() => setLaunchResult(null), 4000);
    }
  };

  const handleLaunchAllNew = async () => {
    if (!selectedSeqId) return;
    const newProspects = prospects.filter((p) => p.status === "new");
    if (newProspects.length === 0) return;
    setSelectedProspects(new Set(newProspects.map((p) => p.id)));
    setLaunching(true);
    setLaunchResult(null);
    try {
      await api.launchSequence(selectedSeqId, newProspects.map((p) => p.id), sendNow);
      setLaunchResult(`Запущено для ${newProspects.length} новых проспектов`);
      setSelectedProspects(new Set());
      api.getProspects().then(setProspects).catch(() => {});
    } catch {
      setLaunchResult("Ошибка запуска");
    } finally {
      setLaunching(false);
      setTimeout(() => setLaunchResult(null), 4000);
    }
  };

  // Fetch steps when a sequence is selected
  useEffect(() => {
    if (!selectedSeqId) {
      queueMicrotask(() => setSteps([]));
      return;
    }
    queueMicrotask(() => setStepsLoading(true));
    api
      .getSequence(selectedSeqId)
      .then((data) => {
        setSteps(data.steps ?? []);
      })
      .catch(() => setSteps([]))
      .finally(() => setStepsLoading(false));
  }, [selectedSeqId]);

  const handleCreateSequence = useCallback(async () => {
    if (!newSeqName.trim()) return;
    try {
      const newSeq = await api.createSequence(newSeqName.trim());
      setSequences((prev) => [...prev, newSeq]);
      setSelectedSeqId(newSeq.id);
      setNewSeqName("");
      setShowNewSeq(false);
    } catch {
      // silently ignore
    }
  }, [newSeqName]);

  const handleAddStep = useCallback(async () => {
    if (!selectedSeqId) return;
    try {
      await api.addStep(selectedSeqId, {
        step_order: steps.length + 1,
        delay_days: addStepDelay,
        prompt_hint: addStepHint || "первое касание",
        channel: addStepChannel,
      });
      const data = await api.getSequence(selectedSeqId);
      setSteps(data.steps ?? []);
      // Reset form
      setShowAddStep(false);
      setAddStepChannel("email");
      setAddStepDelay(0);
      setAddStepHint("первое касание");
    } catch {
      alert("Ошибка добавления шага");
    }
  }, [selectedSeqId, steps, addStepChannel, addStepDelay, addStepHint]);

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
  const newProspectsCount = prospects.filter((p) => p.status === "new").length;

  return (
    <div className="min-h-screen bg-[#f8f9ff]">
      {/* -- Header -- */}
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
          {!showNewSeq ? (
            <button
              onClick={() => setShowNewSeq(true)}
              className="flex items-center gap-2 rounded-xl bg-gradient-to-r from-[#004ac6] to-[#2563eb] px-5 py-2.5 text-sm font-semibold text-white shadow-lg shadow-[#004ac6]/25 transition hover:shadow-xl hover:shadow-[#004ac6]/30"
            >
              <Plus className="size-4" />
              Новая секвенция
            </button>
          ) : (
            <div className="flex items-center gap-2">
              <input
                type="text"
                autoFocus
                value={newSeqName}
                onChange={(e) => setNewSeqName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") handleCreateSequence();
                  if (e.key === "Escape") { setShowNewSeq(false); setNewSeqName(""); }
                }}
                placeholder="Название секвенции..."
                className="rounded-xl border border-slate-200 bg-white px-4 py-2.5 text-sm text-[#0d1c2e] placeholder:text-[#737686] focus:border-[#004ac6] focus:outline-none focus:ring-2 focus:ring-[#004ac6]/20"
              />
              <button
                onClick={handleCreateSequence}
                disabled={!newSeqName.trim()}
                className="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-[#004ac6] to-[#2563eb] px-4 py-2.5 text-sm font-semibold text-white shadow-lg shadow-[#004ac6]/25 transition hover:shadow-xl disabled:opacity-50"
              >
                <Plus className="size-4" />
                Создать
              </button>
              <button
                onClick={() => { setShowNewSeq(false); setNewSeqName(""); }}
                className="rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-sm text-[#434655] transition hover:bg-slate-50"
              >
                <X className="size-4" />
              </button>
            </div>
          )}
        </div>
        {loading && (
          <div className="mt-3 size-5 animate-spin rounded-full border-2 border-[#004ac6] border-t-transparent" />
        )}
      </header>

      {/* -- Bento Grid -- */}
      <div className="grid grid-cols-12 gap-6 px-8 pb-8">
        {/* ====================================================
            LEFT COLUMN (col-span-4): Campaigns + AI tip
           ==================================================== */}
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
                className={`cursor-pointer rounded-2xl bg-white p-5 shadow-sm transition ${
                  isSelected
                    ? "border-2 border-[#22c55e] shadow-md shadow-[#22c55e]/15"
                    : "border border-[#e2e8f0] hover:border-[#004ac6]/30"
                } ${!seq.is_active && !isSelected ? "opacity-70 grayscale" : !seq.is_active ? "opacity-85" : ""}`}
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
            <button
              onClick={() => {
                if (selectedSeqId && steps.length > 0) {
                  const channels: string[] = steps.map(s => s.channel);
                  const allChannels = ["email", "telegram", "phone_call"];
                  const missing = allChannels.filter(c => !channels.includes(c));
                  if (missing.length > 0) {
                    const labels: Record<string, string> = { email: "Email", telegram: "Telegram", phone_call: "Звонок" };
                    alert(`Добавьте шаги с каналами: ${missing.map(c => labels[c] || c).join(", ")}`);
                  } else {
                    alert("Все каналы уже используются — секвенция оптимальна!");
                  }
                } else {
                  alert("Выберите секвенцию и добавьте хотя бы один шаг");
                }
              }}
              className="mt-3 text-xs font-semibold text-[#2f2ebe] hover:underline"
            >
              Оптимизировать сейчас &rarr;
            </button>
          </div>
        </div>

        {/* ====================================================
            MIDDLE COLUMN (col-span-5): Step Timeline
           ==================================================== */}
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
                            <button
                              onClick={async () => {
                                if (!selectedSeqId || !confirm("Удалить шаг?")) return;
                                try {
                                  await api.deleteStep(selectedSeqId, step.id);
                                  const data = await api.getSequence(selectedSeqId);
                                  setSteps(data.steps ?? []);
                                } catch {
                                  alert("Ошибка удаления");
                                }
                              }}
                              className="text-[#737686] hover:text-red-500"
                            >
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
                          <button
                            onClick={async () => {
                              const preview = prompt("Имя проспекта для примера:", "Иван Петров");
                              if (!preview) return;
                              try {
                                const msg = await api.chatWithAI(
                                  `Сгенерируй пример ${step.channel === "email" ? "холодного письма" : step.channel === "telegram" ? "сообщения в Telegram" : "брифа для звонка"} для проспекта "${preview}" компания "Тест". Подсказка: "${step.prompt_hint || "первый контакт"}". Только текст, без пояснений.`,
                                  [],
                                  "sequences"
                                );
                                alert(msg.reply);
                              } catch {
                                alert("Ошибка генерации");
                              }
                            }}
                            className="rounded-lg bg-[#004ac6] px-3 py-1.5 text-xs font-medium text-white transition hover:bg-[#004ac6]/90"
                          >
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
              <div className="mt-6">
                {!showAddStep ? (
                  <button
                    onClick={() => setShowAddStep(true)}
                    className="flex w-full items-center justify-center gap-2 rounded-xl border-2 border-dashed border-slate-200 py-3 text-sm font-medium text-[#434655] transition hover:border-[#004ac6] hover:text-[#004ac6]"
                  >
                    <Plus className="size-4" />
                    Добавить шаг
                  </button>
                ) : (
                  <div className="rounded-xl border border-slate-200 bg-[#eff4ff]/50 p-4">
                    {/* Channel selector */}
                    <p className="mb-2 text-xs font-semibold text-[#434655]">Канал</p>
                    <div className="mb-4 grid grid-cols-3 gap-2">
                      {/* Email */}
                      <button
                        onClick={() => setAddStepChannel("email")}
                        className={`flex flex-col items-center gap-1.5 rounded-xl border-2 p-3 transition ${
                          addStepChannel === "email"
                            ? "border-[#004ac6] bg-blue-50"
                            : "border-slate-200 bg-white hover:border-[#004ac6]/30"
                        }`}
                      >
                        <Mail className={`size-5 ${addStepChannel === "email" ? "text-[#004ac6]" : "text-[#434655]"}`} />
                        <span className={`text-xs font-medium ${addStepChannel === "email" ? "text-[#004ac6]" : "text-[#434655]"}`}>
                          Email
                        </span>
                      </button>
                      {/* Telegram */}
                      <button
                        onClick={() => setAddStepChannel("telegram")}
                        className={`flex flex-col items-center gap-1.5 rounded-xl border-2 p-3 transition ${
                          addStepChannel === "telegram"
                            ? "border-[#3e3fcc] bg-purple-50"
                            : "border-slate-200 bg-white hover:border-[#3e3fcc]/30"
                        }`}
                      >
                        <MessageCircle className={`size-5 ${addStepChannel === "telegram" ? "text-[#3e3fcc]" : "text-[#434655]"}`} />
                        <span className={`text-xs font-medium ${addStepChannel === "telegram" ? "text-[#3e3fcc]" : "text-[#434655]"}`}>
                          Telegram
                        </span>
                      </button>
                      {/* Phone (disabled) */}
                      <div
                        className="relative flex flex-col items-center gap-1.5 rounded-xl border-2 border-slate-200 bg-slate-50 p-3 opacity-50 cursor-not-allowed"
                      >
                        <Phone className="size-5 text-[#434655]" />
                        <span className="text-xs font-medium text-[#434655]">
                          Звонок
                        </span>
                        <span className="absolute -top-2 -right-2 rounded-full bg-orange-100 px-1.5 py-0.5 text-[9px] font-bold text-orange-600">
                          Скоро
                        </span>
                      </div>
                    </div>

                    {/* Delay input */}
                    <div className="mb-3">
                      <label className="mb-1 flex items-center gap-1.5 text-xs font-semibold text-[#434655]">
                        <Clock className="size-3.5" />
                        Задержка (дней)
                      </label>
                      <input
                        type="number"
                        min={0}
                        value={addStepDelay}
                        onChange={(e) => setAddStepDelay(Math.max(0, parseInt(e.target.value) || 0))}
                        className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-[#0d1c2e] focus:border-[#004ac6] focus:outline-none focus:ring-2 focus:ring-[#004ac6]/20"
                      />
                    </div>

                    {/* Prompt hint */}
                    <div className="mb-4">
                      <label className="mb-1 flex items-center gap-1.5 text-xs font-semibold text-[#434655]">
                        <Sparkles className="size-3.5" />
                        Подсказка для AI
                      </label>
                      <input
                        type="text"
                        value={addStepHint}
                        onChange={(e) => setAddStepHint(e.target.value)}
                        placeholder="первое касание"
                        className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-[#0d1c2e] placeholder:text-[#737686] focus:border-[#004ac6] focus:outline-none focus:ring-2 focus:ring-[#004ac6]/20"
                      />
                    </div>

                    {/* Action buttons */}
                    <div className="flex items-center gap-2">
                      <button
                        onClick={handleAddStep}
                        className="flex items-center gap-1.5 rounded-lg bg-[#004ac6] px-4 py-2 text-xs font-semibold text-white transition hover:bg-[#004ac6]/90"
                      >
                        <Plus className="size-3.5" />
                        Добавить
                      </button>
                      <button
                        onClick={() => {
                          setShowAddStep(false);
                          setAddStepChannel("email");
                          setAddStepDelay(0);
                          setAddStepHint("первое касание");
                        }}
                        className="rounded-lg border border-slate-200 bg-white px-4 py-2 text-xs font-medium text-[#434655] transition hover:bg-slate-50"
                      >
                        Отмена
                      </button>
                    </div>
                  </div>
                )}
              </div>
            )}
          </div>
        </div>

        {/* ====================================================
            RIGHT COLUMN (col-span-3): Prospects + Stats
           ==================================================== */}
        <div className="col-span-3 flex flex-col gap-5">
          {/* Prospects */}
          <div className="rounded-2xl bg-[#eff4ff]/50 p-5">
            <div className="mb-3 flex items-center justify-between">
              <h2 className="flex items-center gap-2 text-sm font-semibold text-[#0d1c2e]">
                <Users className="size-4" />
                Проспекты ({prospects.length})
              </h2>
              {prospects.length > 0 && (
                <button
                  onClick={selectAllProspects}
                  className="text-[10px] font-bold text-[#004ac6] hover:underline"
                >
                  {selectedProspects.size === prospects.length ? "Снять все" : "Выбрать все"}
                </button>
              )}
            </div>

            <div className="flex max-h-64 flex-col gap-1 overflow-y-auto">
              {prospects.length === 0 ? (
                <p className="py-6 text-center text-xs text-[#737686]">
                  Нет проспектов. Добавьте в разделе «Проспекты».
                </p>
              ) : (
                prospects.map((p) => (
                  <label
                    key={p.id}
                    className="flex cursor-pointer items-center gap-3 rounded-lg px-3 py-2 transition-colors hover:bg-white/60"
                  >
                    <input
                      type="checkbox"
                      checked={selectedProspects.has(p.id)}
                      onChange={() => toggleProspect(p.id)}
                      className="size-4 rounded border-[#c3c6d7] text-[#004ac6] focus:ring-[#004ac6]/30"
                    />
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium text-[#0d1c2e]">{p.name}</p>
                      <p className="truncate text-[11px] text-[#434655]">{p.company || p.email}</p>
                    </div>
                    <span className={`shrink-0 rounded-full px-2 py-0.5 text-[9px] font-bold uppercase ${
                      p.status === "in_sequence" ? "bg-blue-100 text-blue-700" :
                      p.status === "replied" ? "bg-green-100 text-green-700" :
                      p.status === "converted" ? "bg-purple-100 text-purple-700" :
                      p.status === "opted_out" ? "bg-red-100 text-red-600" :
                      "bg-gray-100 text-gray-600"
                    }`}>
                      {p.status === "new" ? "новый" :
                       p.status === "in_sequence" ? "в секв." :
                       p.status === "replied" ? "ответил" :
                       p.status === "converted" ? "лид" :
                       p.status === "opted_out" ? "отказ" : p.status}
                    </span>
                  </label>
                ))
              )}
            </div>

            {/* Launch section */}
            {selectedSeqId && (
              <div className="relative mt-4">
                {/* Launch options dropdown */}
                {showLaunchOptions && selectedProspects.size > 0 && (
                  <div className="mb-2 rounded-xl border border-slate-200 bg-white p-3 shadow-md">
                    <p className="mb-2 text-xs font-semibold text-[#434655]">Режим запуска</p>
                    <label className="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 transition hover:bg-[#eff4ff]">
                      <input
                        type="radio"
                        name="launch-mode"
                        checked={sendNow}
                        onChange={() => setSendNow(true)}
                        className="size-3.5 text-[#004ac6] focus:ring-[#004ac6]/30"
                      />
                      <Send className="size-3.5 text-[#004ac6]" />
                      <span className="text-xs font-medium text-[#0d1c2e]">Отправить сейчас</span>
                    </label>
                    <label className="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 transition hover:bg-[#eff4ff]">
                      <input
                        type="radio"
                        name="launch-mode"
                        checked={!sendNow}
                        onChange={() => setSendNow(false)}
                        className="size-3.5 text-[#004ac6] focus:ring-[#004ac6]/30"
                      />
                      <Clock className="size-3.5 text-[#434655]" />
                      <span className="text-xs font-medium text-[#0d1c2e]">Запланировать по расписанию</span>
                    </label>
                  </div>
                )}

                {selectedProspects.size > 0 && (
                  <button
                    onClick={() => {
                      if (!showLaunchOptions) {
                        setShowLaunchOptions(true);
                      } else {
                        handleLaunch();
                      }
                    }}
                    disabled={launching}
                    className="flex w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-[#004ac6] to-[#2563eb] py-3 text-sm font-bold text-white shadow-md transition-all hover:-translate-y-0.5 hover:shadow-lg disabled:opacity-50"
                  >
                    {launching ? (
                      <span className="size-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                    ) : showLaunchOptions ? (
                      <Send className="size-4" />
                    ) : (
                      <ChevronDown className="size-4" />
                    )}
                    {launching
                      ? "Генерация..."
                      : showLaunchOptions
                        ? `Запустить (${selectedProspects.size})`
                        : `Запустить (${selectedProspects.size})`}
                  </button>
                )}

                {/* Launch all new button */}
                {newProspectsCount > 0 && (
                  <button
                    onClick={handleLaunchAllNew}
                    disabled={launching}
                    className="mt-2 flex w-full items-center justify-center gap-2 rounded-xl border border-[#004ac6] bg-white py-2.5 text-xs font-semibold text-[#004ac6] transition hover:bg-[#eff4ff] disabled:opacity-50"
                  >
                    <Users className="size-3.5" />
                    Запустить для всех новых ({newProspectsCount})
                  </button>
                )}
              </div>
            )}

            {launchResult && (
              <p className={`mt-2 text-center text-xs font-medium ${launchResult.includes("Ошибка") ? "text-red-500" : "text-green-600"}`}>
                {launchResult}
              </p>
            )}
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
