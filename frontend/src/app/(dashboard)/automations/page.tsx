"use client";

import { Sparkles } from "lucide-react";
import { useAutomations } from "@/hooks/useAutomations";
import { AUTOMATIONS } from "@/components/automations/constants";
import { AutomationCard } from "@/components/automations/AutomationCard";
import { QuickActions } from "@/components/automations/QuickActions";

export default function AutomationsPage() {
  const { toggles, inputs, toggle, toggleAll, updateInput } = useAutomations();

  return (
    <div className="min-h-full px-4 sm:px-6 lg:px-10 pb-12 pt-8 sm:pt-16 lg:pt-24">
      {/* Header */}
      <div className="mb-12 flex flex-col gap-2">
        <div className="flex items-center gap-3">
          <h1 className="text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">
            Автоматизации
          </h1>
          <div className="flex items-center gap-1.5 rounded-full bg-[#e1e0ff] px-3 py-1 text-xs font-bold uppercase tracking-wider text-[#2f2ebe]">
            <Sparkles className="size-3.5" />
            AI Powered
          </div>
        </div>
        <p className="text-lg font-medium text-[#434655]">
          Настройте автоматические действия для вашего отдела продаж
        </p>
      </div>

      {/* Automation Grid */}
      <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
        {AUTOMATIONS.map((auto) => (
          <AutomationCard
            key={auto.id}
            auto={auto}
            isOn={toggles[auto.id]}
            inputValue={inputs[auto.id]}
            onToggle={() => toggle(auto.id)}
            onInputChange={(val) => updateInput(auto.id, val)}
          />
        ))}
      </div>

      <QuickActions toggles={toggles} onToggleAll={toggleAll} />
    </div>
  );
}
