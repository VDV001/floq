import {
  ShieldCheck,
  FileEdit,
  Send,
  Clock,
  ArrowLeftRight,
  FileCheck,
  Zap,
  Brain,
  RefreshCw,
  FileText,
} from "lucide-react";

export interface Automation {
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

export const AUTOMATIONS: Automation[] = [
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

import type { UserSettings } from "@/lib/api";

export const TOGGLE_MAP: Record<string, keyof UserSettings> = {
  "auto-qualify": "auto_qualify",
  "auto-draft": "auto_draft",
  "auto-send": "auto_send",
  "auto-followup": "auto_followup",
  "prospect-to-lead": "auto_prospect_to_lead",
  "verify-import": "auto_verify_import",
};

export const INPUT_MAP: Record<string, keyof UserSettings> = {
  "auto-send": "auto_send_delay_min",
  "auto-followup": "auto_followup_days",
};
