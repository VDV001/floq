import { MinusCircle, CheckCircle2, AlertTriangle, XCircle } from "lucide-react";

export type ProspectStatus = "Новый" | "В секвенции" | "Ответил" | "Конвертирован" | "Отписался";

export interface UIProspect {
  initials: string;
  avatarColor: string;
  name: string;
  company: string;
  position: string;
  email: string;
  phone: string;
  whatsapp: string;
  telegramUsername: string;
  sourceName: string;
  status: ProspectStatus;
  verifyStatus: "not_checked" | "valid" | "risky" | "invalid";
  verifyScore: number;
}

export function mapProspectStatus(s: string): ProspectStatus {
  const m: Record<string, ProspectStatus> = {
    new: "Новый",
    in_sequence: "В секвенции",
    replied: "Ответил",
    converted: "Конвертирован",
    opted_out: "Отписался",
  };
  return m[s] || "Новый";
}

export function mapProspects(data: { name: string; company: string; title: string; email: string; phone: string; whatsapp: string; telegram_username: string; source_name?: string; status: string; verify_status: string; verify_score: number }[]): UIProspect[] {
  return data.map((p) => ({
    initials: p.name.split(" ").map((w) => w[0]).join("").toUpperCase().slice(0, 2),
    avatarColor: "bg-[#d8e3fb]",
    name: p.name,
    company: p.company,
    position: p.title,
    email: p.email,
    phone: p.phone || "",
    whatsapp: p.whatsapp || "",
    telegramUsername: p.telegram_username || "",
    sourceName: p.source_name || "",
    status: mapProspectStatus(p.status),
    verifyStatus: p.verify_status as UIProspect["verifyStatus"],
    verifyScore: p.verify_score,
  }));
}

export const STATUS_STYLES: Record<ProspectStatus, string> = {
  Новый: "bg-blue-100 text-blue-700",
  "В секвенции": "bg-purple-100 text-purple-700",
  Ответил: "bg-green-100 text-green-700",
  Конвертирован: "bg-green-600 text-white",
  Отписался: "bg-slate-200 text-slate-600",
};

export const VERIFY_STYLES: Record<string, { text: string; icon: typeof MinusCircle; label: string }> = {
  not_checked: { text: "text-gray-500", icon: MinusCircle, label: "Не проверен" },
  valid: { text: "text-green-700", icon: CheckCircle2, label: "Валидный" },
  risky: { text: "text-yellow-700", icon: AlertTriangle, label: "Рискованный" },
  invalid: { text: "text-red-700", icon: XCircle, label: "Невалидный" },
};
