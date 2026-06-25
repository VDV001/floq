import { useState, useEffect, useCallback } from "react";
import { api, type Prospect } from "@/lib/api";
import { useNotify } from "@/components/notifications/NotificationProvider";

export function useProspects() {
  const { notify, notifyError } = useNotify();
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
        notify({
          type: "success",
          title: "Письма подготовлены",
          message: `Создано писем для ${prospectIds.length} проспектов.`,
          remedy: "Проверьте и одобрите их к отправке в разделе «Исходящие».",
          action: { label: "Открыть Исходящие", href: "/outbound" },
        });
        api.getProspects().then(setProspects).catch(() => {});
      } catch (err) {
        setLaunchResult("Ошибка запуска");
        notifyError(err, "Не удалось запустить отправку");
      } finally {
        setLaunching(false);
        setTimeout(() => setLaunchResult(null), 4000);
      }
    },
    [notify, notifyError]
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
