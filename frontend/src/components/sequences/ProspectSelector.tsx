import { useState } from "react";
import { Users, Send, Clock, ChevronDown } from "lucide-react";
import type { Prospect } from "@/lib/api";

interface ProspectSelectorProps {
  prospects: Prospect[];
  selectedProspects: Set<string>;
  selectedSeqId: string | null;
  launching: boolean;
  launchResult: string | null;
  newProspectsCount: number;
  onToggle: (id: string) => void;
  onSelectAll: () => void;
  onLaunch: (prospectIds: string[], sendNow: boolean) => void;
  onLaunchAllNew: (sendNow: boolean) => void;
}

const STATUS_STYLES: Record<string, { className: string; label: string }> = {
  new: { className: "bg-gray-100 text-gray-600", label: "новый" },
  in_sequence: { className: "bg-blue-100 text-blue-700", label: "в секв." },
  replied: { className: "bg-green-100 text-green-700", label: "ответил" },
  converted: { className: "bg-purple-100 text-purple-700", label: "лид" },
  opted_out: { className: "bg-red-100 text-red-600", label: "отказ" },
};

export function ProspectSelector({
  prospects,
  selectedProspects,
  selectedSeqId,
  launching,
  launchResult,
  newProspectsCount,
  onToggle,
  onSelectAll,
  onLaunch,
  onLaunchAllNew,
}: ProspectSelectorProps) {
  const [showLaunchOptions, setShowLaunchOptions] = useState(false);
  const [sendNow, setSendNow] = useState(true);

  return (
    <div className="rounded-2xl bg-[#eff4ff]/50 p-5">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="flex items-center gap-2 text-sm font-semibold text-[#0d1c2e]">
          <Users className="size-4" />
          Проспекты ({prospects.length})
        </h2>
        {prospects.length > 0 && (
          <button onClick={onSelectAll} className="text-[10px] font-bold text-[#004ac6] hover:underline">
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
          prospects.map((p) => {
            const status = STATUS_STYLES[p.status] ?? { className: "bg-gray-100 text-gray-600", label: p.status };
            return (
              <label
                key={p.id}
                className="flex cursor-pointer items-center gap-3 rounded-lg px-3 py-2 transition-colors hover:bg-white/60"
              >
                <input
                  type="checkbox"
                  checked={selectedProspects.has(p.id)}
                  onChange={() => onToggle(p.id)}
                  className="size-4 rounded border-[#c3c6d7] text-[#004ac6] focus:ring-[#004ac6]/30"
                />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-[#0d1c2e]">{p.name}</p>
                  <p className="truncate text-[11px] text-[#434655]">{p.company || p.email}</p>
                </div>
                <span className={`shrink-0 rounded-full px-2 py-0.5 text-[9px] font-bold uppercase ${status.className}`}>
                  {status.label}
                </span>
              </label>
            );
          })
        )}
      </div>

      {selectedSeqId && (
        <div className="relative mt-4">
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
                  setShowLaunchOptions(false);
                  onLaunch(Array.from(selectedProspects), sendNow);
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
              {launching ? "Генерация..." : `Запустить (${selectedProspects.size})`}
            </button>
          )}

          {newProspectsCount > 0 && (
            <button
              onClick={() => onLaunchAllNew(sendNow)}
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
  );
}
