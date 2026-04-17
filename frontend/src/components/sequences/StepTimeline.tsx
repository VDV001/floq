import { useState } from "react";
import { Plus, Layers, Copy, Trash2 } from "lucide-react";
import type { SequenceStep } from "@/lib/api";
import { CHANNEL_LABELS } from "./constants";
import { StepPreview } from "./StepPreview";
import { AddStepForm } from "./AddStepForm";

interface StepTimelineProps {
  selectedSeqId: string | null;
  selectedSequenceName: string | null;
  steps: SequenceStep[];
  stepsLoading: boolean;
  onDeleteStep: (stepId: string) => void;
  onAddStep: (params: { channel: "email" | "telegram"; delay_days: number; prompt_hint: string }) => Promise<void>;
  onConfirmDelete: (title: string, message: string, onConfirm: () => void) => void;
}

export function StepTimeline({
  selectedSeqId,
  selectedSequenceName,
  steps,
  stepsLoading,
  onDeleteStep,
  onAddStep,
  onConfirmDelete,
}: StepTimelineProps) {
  const [previewStepId, setPreviewStepId] = useState<string | null>(null);
  const [showAddStep, setShowAddStep] = useState(false);

  return (
    <div className="rounded-2xl bg-white p-6 shadow-sm">
      <h2 className="mb-6 text-base font-semibold text-[#0d1c2e]">
        Шаги секвенции
        {selectedSequenceName && (
          <span className="ml-2 text-sm font-normal text-[#737686]">— {selectedSequenceName}</span>
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
          <p className="text-sm text-[#737686]">Выберите секвенцию слева</p>
        </div>
      )}

      {!stepsLoading && selectedSeqId && steps.length === 0 && (
        <div className="py-12 text-center">
          <Plus className="mx-auto mb-3 size-8 text-[#c3c6d7]" />
          <p className="text-sm text-[#737686]">Нет шагов в этой секвенции</p>
          <p className="mt-1 text-xs text-[#737686]">Добавьте первый шаг, чтобы начать</p>
        </div>
      )}

      {!stepsLoading && steps.length > 0 && (
        <div className="relative ml-3">
          <div className="absolute left-[7px] top-0 bottom-0 w-0.5 bg-slate-200" />
          {steps
            .sort((a, b) => a.step_order - b.step_order)
            .map((step, idx) => {
              const isFirst = idx === 0;
              const isLast = idx === steps.length - 1;
              const ch = CHANNEL_LABELS[step.channel] ?? CHANNEL_LABELS.email;
              const dayNum = steps.slice(0, idx + 1).reduce((sum, s) => sum + s.delay_days, 0);

              return (
                <div key={step.id} className={`relative pl-8 ${isLast ? "" : "mb-8"}`}>
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
                            Задержка: {step.delay_days} {step.delay_days === 1 ? "день" : "дней"}
                          </span>
                        )}
                      </div>
                      <p className="mt-1 text-sm font-medium text-[#0d1c2e]">День {dayNum}</p>
                    </div>
                    <div className="flex items-center gap-3">
                      <span className={`rounded-full ${ch.bg} px-2.5 py-0.5 text-xs font-medium ${ch.color}`}>
                        {ch.label}
                      </span>
                      <button className="text-[#737686] hover:text-[#0d1c2e]">
                        <Copy className="size-3.5" />
                      </button>
                      <button
                        onClick={() => {
                          onConfirmDelete(
                            "Удалить шаг",
                            `Удалить шаг ${step.step_order} (${CHANNEL_LABELS[step.channel]?.label || step.channel})?`,
                            () => onDeleteStep(step.id)
                          );
                        }}
                        className="text-[#737686] hover:text-red-500"
                      >
                        <Trash2 className="size-3.5" />
                      </button>
                    </div>
                  </div>

                  {step.prompt_hint && (
                    <p className="mt-2 text-xs leading-relaxed text-[#434655]">{step.prompt_hint}</p>
                  )}

                  <div className="mt-3">
                    <button
                      onClick={() => { setPreviewStepId(step.id); }}
                      className="rounded-lg bg-[#004ac6] px-3 py-1.5 text-xs font-medium text-white transition hover:bg-[#004ac6]/90"
                    >
                      Сгенерировать пример
                    </button>
                  </div>

                  {previewStepId === step.id && (
                    <StepPreview
                      channel={step.channel}
                      promptHint={step.prompt_hint || "первое касание"}
                      onClose={() => setPreviewStepId(null)}
                    />
                  )}
                </div>
              );
            })}
        </div>
      )}

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
            <AddStepForm
              onAdd={async (params) => {
                await onAddStep(params);
                setShowAddStep(false);
              }}
              onCancel={() => setShowAddStep(false)}
            />
          )}
        </div>
      )}
    </div>
  );
}
