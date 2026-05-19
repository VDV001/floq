import { Star, CheckCircle2, MessageCircle, RotateCcw, CircleCheck } from "lucide-react";
import { STATUS_STYLES, type LeadStatus } from "@/components/leads/constants";

// Re-export lead-domain primitives so existing inbox callers don't have
// to chase the move. New code should import from
// @/components/leads/constants directly.
export { STATUS_STYLES, type LeadStatus };

export interface InboxLead {
  id: string;
  company: string;
  contact: string;
  channel: "email" | "telegram";
  preview: string;
  timeAgo: string;
  status: LeadStatus;
  apiStatus: string;
  sourceName?: string;
  pendingRepliesCount: number;
}

export const PIPELINE_STAGES_CONFIG: { id: string; apiStatus: string; label: string; icon: typeof Star; alert?: boolean }[] = [
  { id: "new", apiStatus: "new", label: "Новые лиды", icon: Star },
  { id: "qualified", apiStatus: "qualified", label: "Квалифицированные", icon: CheckCircle2 },
  { id: "conversation", apiStatus: "in_conversation", label: "В диалоге", icon: MessageCircle },
  { id: "followup", apiStatus: "followup", label: "Фоллоуап", icon: RotateCcw, alert: true },
  { id: "closed", apiStatus: "closed", label: "Закрытые", icon: CircleCheck },
];

export const FILTER_TABS = ["Все", "Непрочитанные", "Приоритетные"] as const;

export function mapStatus(status: string): LeadStatus {
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
