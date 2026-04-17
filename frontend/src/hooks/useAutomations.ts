import { useState, useEffect, useRef, useCallback } from "react";
import { api } from "@/lib/api";
import { AUTOMATIONS, TOGGLE_MAP, INPUT_MAP } from "@/components/automations/constants";

export function useAutomations() {
  const [toggles, setToggles] = useState<Record<string, boolean>>(
    Object.fromEntries(AUTOMATIONS.map((a) => [a.id, a.defaultOn]))
  );
  const [inputs, setInputs] = useState<Record<string, number>>({
    "auto-send": 5,
    "auto-followup": 2,
  });
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    api.getSettings().then((s) => {
      const newToggles: Record<string, boolean> = {};
      for (const [autoId, field] of Object.entries(TOGGLE_MAP)) {
        newToggles[autoId] = (s[field] as boolean) ?? AUTOMATIONS.find(a => a.id === autoId)?.defaultOn ?? false;
      }
      setToggles(newToggles);
      setInputs({
        "auto-send": s.auto_send_delay_min || 5,
        "auto-followup": s.auto_followup_days || 2,
      });
    }).catch(() => {});
  }, []);

  const saveToAPI = useCallback((newToggles: Record<string, boolean>, newInputs: Record<string, number>) => {
    if (saveTimer.current !== null) clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(() => {
      const data: Record<string, unknown> = {};
      for (const [autoId, field] of Object.entries(TOGGLE_MAP)) {
        data[field] = newToggles[autoId];
      }
      for (const [autoId, field] of Object.entries(INPUT_MAP)) {
        data[field] = newInputs[autoId];
      }
      api.updateSettings(data as Partial<import("@/lib/api").UserSettings>).catch(() => {});
    }, 500);
  }, []);

  const toggle = (id: string) => {
    setToggles((prev) => {
      const next = { ...prev, [id]: !prev[id] };
      saveToAPI(next, inputs);
      return next;
    });
  };

  const toggleAll = () => {
    const allOn = Object.values(toggles).every(Boolean);
    const next = Object.fromEntries(
      AUTOMATIONS.map((a) => [a.id, !allOn])
    );
    setToggles(next);
    saveToAPI(next, inputs);
  };

  const updateInput = (autoId: string, val: number) => {
    setInputs((prev) => {
      const next = { ...prev, [autoId]: val };
      saveToAPI(toggles, next);
      return next;
    });
  };

  return { toggles, inputs, toggle, toggleAll, updateInput };
}
