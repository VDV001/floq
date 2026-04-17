import { useState, useEffect, useCallback } from "react";
import { api, type SequenceStep } from "@/lib/api";

export function useSequenceSteps(selectedSeqId: string | null) {
  const [steps, setSteps] = useState<SequenceStep[]>([]);
  const [stepsLoading, setStepsLoading] = useState(false);

  useEffect(() => {
    if (!selectedSeqId) {
      queueMicrotask(() => setSteps([]));
      return;
    }
    queueMicrotask(() => setStepsLoading(true));
    api
      .getSequence(selectedSeqId)
      .then((data) => setSteps(data.steps ?? []))
      .catch(() => setSteps([]))
      .finally(() => setStepsLoading(false));
  }, [selectedSeqId]);

  const addStep = useCallback(
    async (params: { channel: "email" | "telegram"; delay_days: number; prompt_hint: string }) => {
      if (!selectedSeqId) return;
      await api.addStep(selectedSeqId, {
        step_order: steps.length + 1,
        delay_days: params.delay_days,
        prompt_hint: params.prompt_hint,
        channel: params.channel,
      });
      const data = await api.getSequence(selectedSeqId);
      setSteps(data.steps ?? []);
    },
    [selectedSeqId, steps.length]
  );

  const deleteStep = useCallback(
    async (stepId: string) => {
      if (!selectedSeqId) return;
      await api.deleteStep(selectedSeqId, stepId);
      const data = await api.getSequence(selectedSeqId);
      setSteps(data.steps ?? []);
    },
    [selectedSeqId]
  );

  return { steps, stepsLoading, addStep, deleteStep };
}
