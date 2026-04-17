import { useState, useEffect, useCallback } from "react";
import { api, type Prospect } from "@/lib/api";

export function useProspects() {
  const [prospects, setProspects] = useState<Prospect[]>([]);
  const [selectedProspects, setSelectedProspects] = useState<Set<string>>(new Set());
  const [launching, setLaunching] = useState(false);
  const [launchResult, setLaunchResult] = useState<string | null>(null);

  useEffect(() => {
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

  const launchSequence = useCallback(
    async (seqId: string, prospectIds: string[], sendNow: boolean) => {
      setLaunching(true);
      setLaunchResult(null);
      try {
        await api.launchSequence(seqId, prospectIds, sendNow);
        setLaunchResult(`Запущено для ${prospectIds.length} проспектов`);
        setSelectedProspects(new Set());
        api.getProspects().then(setProspects).catch(() => {});
      } catch {
        setLaunchResult("Ошибка запуска");
      } finally {
        setLaunching(false);
        setTimeout(() => setLaunchResult(null), 4000);
      }
    },
    []
  );

  const newProspectsCount = prospects.filter((p) => p.status === "new").length;

  return {
    prospects,
    selectedProspects,
    launching,
    launchResult,
    newProspectsCount,
    toggleProspect,
    selectAllProspects,
    launchSequence,
  };
}
