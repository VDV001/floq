import { Star, CheckCircle2, MessageCircle, RotateCcw, CircleCheck } from "lucide-react";

export interface InboxLead {
  id: string;
  company: string;
  contact: string;
  channel: "email" | "telegram";
  preview: string;
  timeAgo: string;
  status: "Новый" | "Квалифицирован" | "В диалоге" | "Нужен фоллоуап" | "Закрыт" | "Выигран";
  apiStatus: string;
  sourceName?: string;
}

export const STATUS_STYLES: Record<InboxLead["status"], string> = {
  "Новый": "bg-[#dbe1ff] text-[#004ac6]",
  "Квалифицирован": "bg-[#c7d2fe] text-[#3730a3]",
  "В диалоге": "bg-[#fef3c7] text-[#92400e]",
  "Нужен фоллоуап": "bg-[#fee2e2] text-[#dc2626]",
  "Закрыт": "bg-[#d1fae5] text-[#065f46]",
  "Выигран": "bg-[#bbf7d0] text-[#14532d]",
};

export const PIPELINE_STAGES_CONFIG: { id: string; apiStatus: string; label: string; icon: typeof Star; alert?: boolean }[] = [
  { id: "new", apiStatus: "new", label: "Новые лиды", icon: Star },
  { id: "qualified", apiStatus: "qualified", label: "Квалифицированные", icon: CheckCircle2 },
  { id: "conversation", apiStatus: "in_conversation", label: "В диалоге", icon: MessageCircle },
  { id: "followup", apiStatus: "followup", label: "Фоллоуап", icon: RotateCcw, alert: true },
  { id: "closed", apiStatus: "closed", label: "Закрытые", icon: CircleCheck },
];

export const FILTER_TABS = ["Все", "Непрочитанные", "Приоритетные"] as const;

export function mapStatus(status: string): InboxLead["status"] {
  switch (status) {
    case "new": return "Новый";
    case "qualified": return "Квалифицирован";
    case "in_conversation": return "В диалоге";
    case "followup": return "Нужен фоллоуап";
    case "closed": return "Закрыт";
    case "won": return "Выигран";
    default: return "Новый";
  }
}
