import { Lightbulb } from "lucide-react";
import { AUTOMATIONS } from "./constants";

interface QuickActionsProps {
  toggles: Record<string, boolean>;
  onToggleAll: () => void;
}

export function QuickActions({ toggles, onToggleAll }: QuickActionsProps) {
  const allOn = Object.values(toggles).every(Boolean);

  return (
    <div className="group relative mt-12 overflow-hidden rounded-xl bg-[#e1e0ff] p-8">
      <div className="absolute -mr-20 -mt-20 right-0 top-0 size-64 rounded-full bg-[#585be6]/10 blur-3xl transition-transform duration-700 group-hover:scale-110" />
      <div className="relative z-10 flex flex-col items-start gap-8 md:flex-row md:items-center">
        <div className="flex size-16 shrink-0 items-center justify-center rounded-2xl bg-white text-[#3e3fcc] shadow-lg">
          <Lightbulb className="size-8" />
        </div>
        <div className="flex-1">
          <h2 className="mb-2 text-xl font-bold text-[#07006c]">
            Быстрые действия
          </h2>
          <p className="max-w-2xl font-medium leading-relaxed text-[#2f2ebe]">
            {allOn
              ? "Все автоматизации включены. Система работает на максимум."
              : `Включено ${Object.values(toggles).filter(Boolean).length} из ${AUTOMATIONS.length} автоматизаций. Включите все для максимальной эффективности.`}
          </p>
        </div>
        <button
          onClick={onToggleAll}
          className="whitespace-nowrap rounded-xl bg-[#3e3fcc] px-6 py-3 font-bold text-white transition-all hover:bg-[#585be6] hover:shadow-lg active:scale-95"
        >
          {allOn ? "Выключить все" : "Включить все"}
        </button>
      </div>
    </div>
  );
}
