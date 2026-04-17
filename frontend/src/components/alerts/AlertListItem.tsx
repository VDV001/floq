import { Sparkles, X, Mail, Calendar } from "lucide-react";
import type { Lead } from "@/lib/api";
import { getTimeAgo, getInitials } from "./helpers";

interface AlertListItemProps {
  alert: Lead;
}

export function AlertListItem({ alert }: AlertListItemProps) {
  return (
    <div
      className="flex flex-wrap items-center justify-between rounded-xl bg-white p-6 transition-all duration-300 hover:bg-[#eff4ff]"
    >
      {/* Left: contact */}
      <div className="flex min-w-[300px] flex-1 items-center gap-5">
        <div className="flex size-12 shrink-0 items-center justify-center rounded-xl bg-[#d5e0f8] font-black text-[#434655]">
          {getInitials(alert.contact_name)}
        </div>
        <div>
          <h4 className="text-lg font-bold text-[#0d1c2e]">
            {alert.contact_name}
          </h4>
          <p className="text-sm font-medium text-[#434655]">
            {alert.company || "—"} ·{" "}
            {alert.channel === "telegram" ? "Telegram" : "Email"}
          </p>
        </div>
        <div className="ml-4 hidden border-l border-[#c3c6d7]/20 px-4 py-2 md:block">
          <p className="mb-1 text-[0.65rem] font-bold uppercase tracking-widest text-[#434655]">
            Последний контакт
          </p>
          <p className="text-sm font-bold text-[#0d1c2e]">
            {getTimeAgo(alert.updated_at)} назад
          </p>
        </div>
      </div>

      {/* Middle: first message preview */}
      <div className="hidden min-w-[400px] flex-1 px-8 lg:block">
        <div className="flex items-center gap-3">
          <Sparkles className="size-5 shrink-0 text-[#3e3fcc]" />
          <p className="text-sm font-semibold text-[#434655]">
            <span className="font-bold text-[#0d1c2e]">Действие:</span>{" "}
            {alert.first_message
              ? `Напомнить о: "${alert.first_message.slice(0, 80)}${alert.first_message.length > 80 ? "..." : ""}"`
              : "Связаться с лидом для продолжения диалога."}
          </p>
        </div>
      </div>

      {/* Right: buttons */}
      <div className="flex items-center gap-2">
        <button className="flex size-10 items-center justify-center rounded-lg bg-[#2563eb] text-white shadow-sm transition-all hover:scale-105">
          <Mail className="size-[18px]" />
        </button>
        <button className="flex size-10 items-center justify-center rounded-lg text-[#434655] transition-all hover:bg-white hover:shadow-sm">
          <Calendar className="size-[18px]" />
        </button>
        <button className="flex size-10 items-center justify-center rounded-lg text-[#434655] transition-all hover:text-[#ba1a1a]">
          <X className="size-[18px]" />
        </button>
      </div>
    </div>
  );
}
