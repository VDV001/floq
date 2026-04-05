"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { api } from "@/lib/api";
import {
  ShieldCheck,
  FileEdit,
  Send,
  Clock,
  ArrowLeftRight,
  FileCheck,
  Zap,
  Brain,
  Sparkles,
  RefreshCw,
  FileText,
  Lightbulb,
} from "lucide-react";
import { Switch } from "@/components/ui/switch";

/* ------------------------------------------------------------------ */
/*  Automation config                                                  */
/* ------------------------------------------------------------------ */

interface Automation {
  id: string;
  icon: typeof ShieldCheck;
  iconBg: string;
  iconColor: string;
  title: string;
  description: string;
  defaultOn: boolean;
  bottom:
    | { type: "tag"; icon: typeof Zap; text: string; color: string }
    | { type: "input"; label: string; defaultValue: number };
}

const AUTOMATIONS: Automation[] = [
  {
    id: "auto-qualify",
    icon: ShieldCheck,
    iconBg: "bg-[#dbe1ff]/30",
    iconColor: "text-[#004ac6]",
    title: "Авто-квалификация",
    description:
      "AI автоматически оценивает новые лиды на основе истории успешных сделок.",
    defaultOn: true,
    bottom: {
      type: "tag",
      icon: Zap,
      text: "Мгновенное выполнение",
      color: "text-[#004ac6]",
    },
  },
  {
    id: "auto-draft",
    icon: FileEdit,
    iconBg: "bg-[#e1e0ff]/40",
    iconColor: "text-[#3e3fcc]",
    title: "Авто-черновик",
    description:
      "ИИ создает черновик персонализированного ответа для всех квалифицированных лидов.",
    defaultOn: true,
    bottom: {
      type: "tag",
      icon: Brain,
      text: "Персонализация включена",
      color: "text-[#3e3fcc]",
    },
  },
  {
    id: "auto-send",
    icon: Send,
    iconBg: "bg-[#dce9ff]",
    iconColor: "text-[#434655] group-hover:text-[#004ac6] transition-colors",
    title: "Авто-отправка email",
    description: "Утвержденные сообщения отправляются автоматически.",
    defaultOn: false,
    bottom: { type: "input", label: "Задержка (мин)", defaultValue: 5 },
  },
  {
    id: "auto-followup",
    icon: Clock,
    iconBg: "bg-[#d5e0f8]/40",
    iconColor: "text-[#545f73]",
    title: "Авто-фоллоуап",
    description:
      "ИИ отправляет напоминание через заданное время, если нет ответа.",
    defaultOn: true,
    bottom: {
      type: "input",
      label: "Дней до напоминания",
      defaultValue: 2,
    },
  },
  {
    id: "prospect-to-lead",
    icon: ArrowLeftRight,
    iconBg: "bg-[#dbe1ff]/30",
    iconColor: "text-[#004ac6]",
    title: "Проспект → Лид",
    description:
      'Автоматическая конвертация проспекта в статус "Лид" при первом ответе.',
    defaultOn: true,
    bottom: {
      type: "tag",
      icon: RefreshCw,
      text: "Синхронизация CRM",
      color: "text-[#004ac6]",
    },
  },
  {
    id: "verify-import",
    icon: FileCheck,
    iconBg: "bg-[#dce9ff]",
    iconColor: "text-[#434655]",
    title: "Верификация при импорте",
    description:
      "Автоматическая проверка валидности email адресов при загрузке CSV файлов.",
    defaultOn: false,
    bottom: {
      type: "tag",
      icon: FileText,
      text: "Поддержка CSV/XLSX",
      color: "text-[#737686]",
    },
  },
];

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

// Mapping from automation IDs to settings fields
const TOGGLE_MAP: Record<string, string> = {
  "auto-qualify": "auto_qualify",
  "auto-draft": "auto_draft",
  "auto-send": "auto_send",
  "auto-followup": "auto_followup",
  "prospect-to-lead": "auto_prospect_to_lead",
  "verify-import": "auto_verify_import",
};

const INPUT_MAP: Record<string, string> = {
  "auto-send": "auto_send_delay_min",
  "auto-followup": "auto_followup_days",
};

export default function AutomationsPage() {
  const [toggles, setToggles] = useState<Record<string, boolean>>(
    Object.fromEntries(AUTOMATIONS.map((a) => [a.id, a.defaultOn]))
  );
  const [inputs, setInputs] = useState<Record<string, number>>({
    "auto-send": 5,
    "auto-followup": 2,
  });
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Load settings from API on mount
  useEffect(() => {
    api.getSettings().then((s) => {
      const newToggles: Record<string, boolean> = {};
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const sAny = s as any;
      for (const [autoId, field] of Object.entries(TOGGLE_MAP)) {
        newToggles[autoId] = (sAny[field] as boolean) ?? AUTOMATIONS.find(a => a.id === autoId)?.defaultOn ?? false;
      }
      setToggles(newToggles);
      setInputs({
        "auto-send": s.auto_send_delay_min || 5,
        "auto-followup": s.auto_followup_days || 2,
      });
    }).catch(() => {});
  }, []);

  // Debounced save to API
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

  return (
    <div className="min-h-full px-4 sm:px-6 lg:px-10 pb-12 pt-8 sm:pt-16 lg:pt-24">
      {/* Header */}
      <div className="mb-12 flex flex-col gap-2">
        <div className="flex items-center gap-3">
          <h1 className="text-2xl sm:text-3xl lg:text-4xl font-extrabold tracking-tight text-[#0d1c2e]">
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
        {AUTOMATIONS.map((auto) => {
          const Icon = auto.icon;
          const isOn = toggles[auto.id];
          return (
            <div
              key={auto.id}
              className="group rounded-xl border border-[#e5e7eb] bg-white p-6 transition-all duration-300 hover:shadow-xl hover:shadow-[#004ac6]/5"
            >
              {/* Top: icon + toggle */}
              <div className="mb-6 flex items-start justify-between">
                <div
                  className={`flex size-12 items-center justify-center rounded-lg ${auto.iconBg}`}
                >
                  <Icon className={`size-6 ${auto.iconColor}`} />
                </div>
                <Switch checked={isOn} onCheckedChange={() => toggle(auto.id)} />
              </div>

              {/* Title + description */}
              <h3 className="mb-2 text-lg font-bold text-[#0d1c2e]">
                {auto.title}
              </h3>
              <p className="text-sm leading-relaxed text-[#434655]">
                {auto.description}
              </p>

              {/* Bottom area */}
              {auto.bottom.type === "tag" ? (
                <div className="mt-6 flex items-center gap-2 border-t border-[#c3c6d7]/10 pt-4 text-xs font-semibold">
                  {(() => {
                    const TagIcon = auto.bottom.icon;
                    return (
                      <>
                        <TagIcon className={`size-3.5 ${auto.bottom.color}`} />
                        <span className={auto.bottom.color}>
                          {auto.bottom.text}
                        </span>
                      </>
                    );
                  })()}
                </div>
              ) : (
                <div className="mt-4 flex flex-col gap-2 rounded-lg bg-[#f8f9ff] p-3">
                  <label className="text-[10px] font-bold uppercase tracking-wider text-[#434655]">
                    {auto.bottom.label}
                  </label>
                  <input
                    type="number"
                    value={inputs[auto.id] ?? auto.bottom.defaultValue}
                    onChange={(e) => {
                      const val = Number(e.target.value);
                      setInputs((prev) => {
                        const next = { ...prev, [auto.id]: val };
                        saveToAPI(toggles, next);
                        return next;
                      });
                    }}
                    className="rounded border border-[#c3c6d7]/30 bg-white px-2 py-1 text-sm outline-none focus:border-[#004ac6] focus:ring-1 focus:ring-[#004ac6]"
                  />
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* Quick actions */}
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
              {Object.values(toggles).every(Boolean)
                ? "Все автоматизации включены. Система работает на максимум."
                : `Включено ${Object.values(toggles).filter(Boolean).length} из ${AUTOMATIONS.length} автоматизаций. Включите все для максимальной эффективности.`}
            </p>
          </div>
          <button
            onClick={() => {
              const allOn = Object.values(toggles).every(Boolean);
              const next = Object.fromEntries(
                AUTOMATIONS.map((a) => [a.id, !allOn])
              );
              setToggles(next);
              saveToAPI(next, inputs);
            }}
            className="whitespace-nowrap rounded-xl bg-[#3e3fcc] px-6 py-3 font-bold text-white transition-all hover:bg-[#585be6] hover:shadow-lg active:scale-95"
          >
            {Object.values(toggles).every(Boolean)
              ? "Выключить все"
              : "Включить все"}
          </button>
        </div>
      </div>
    </div>
  );
}
