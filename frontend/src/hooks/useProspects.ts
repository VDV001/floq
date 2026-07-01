import { useState, useEffect, useCallback } from "react";
import { api, type Prospect } from "@/lib/api";
import { useNotify } from "@/components/notifications/NotificationProvider";

export function useProspects() {
  const { notify, notifyError } = useNotify();
  const [prospects, setProspects] = useState<Prospect[]>([]);
  const [selectedProspects, setSelectedProspects] = useState<Set<string>>(new Set());
  const [launching, setLaunching] = useState(false);

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
      try {
        const res = await api.launchSequence(seqId, prospectIds, sendNow);
        setSelectedProspects(new Set());
        const queued = res?.queued ?? 0;
        const skipped = res?.skipped ?? 0;
        if (queued === 0) {
          // Nothing went out — never report a false success. Tell the user why
          // (skipped prospects) so they can fix it (usually email verification).
          notify({
            type: "error",
            title: "Ничего не отправлено",
            message:
              skipped > 0
                ? `Ни одного письма не создано. Пропущено проспектов: ${skipped} — не прошли проверку email.`
                : "Ни одного письма не создано.",
            remedy: "Проверьте верификацию email проспектов и их статусы, затем запустите снова.",
          });
        } else {
          notify({
            type: "success",
            title: "Письма подготовлены",
            message:
              skipped > 0
                ? `Создано писем для ${queued} проспектов. Пропущено ${skipped} — не прошли проверку email.`
                : `Создано писем для ${queued} проспектов.`,
            remedy: "Проверьте и одобрите их к отправке в разделе «Исходящие».",
            action: { label: "Открыть Исходящие", href: "/outbound" },
          });
        }
        api.getProspects().then(setProspects).catch(() => {});
      } catch (err) {
        notifyError(err, "Не удалось запустить отправку");
      } finally {
        setLaunching(false);
      }
    },
    [notify, notifyError]
  );

  const newProspectsCount = prospects.filter((p) => p.status === "new").length;

  return {
    prospects,
    selectedProspects,
    launching,
    newProspectsCount,
    toggleProspect,
    selectAllProspects,
    launchSequence,
  };
}
