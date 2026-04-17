import { Clock, Send, Sparkles, X } from "lucide-react";
import type { Lead } from "@/lib/api";
import { getTimeAgo, getInitials, getSilentDays } from "./helpers";

interface FeaturedCardProps {
  featured: Lead;
}

export function FeaturedCard({ featured }: FeaturedCardProps) {
  return (
    <div className="xl:col-span-2">
      <div className="relative overflow-hidden rounded-xl bg-white p-8 transition-all duration-300 hover:shadow-[0_12px_40px_rgba(13,28,46,0.06)]">
        {/* Silent badge */}
        <div className="absolute right-6 top-6">
          <span className="flex items-center gap-1 text-[0.7rem] font-black uppercase tracking-tight text-[#ba1a1a]">
            <Clock className="size-3.5" />
            Молчит {getSilentDays(featured.updated_at)} д
          </span>
        </div>

        {/* Contact info */}
        <div className="mb-8 flex items-start gap-6">
          <div className="flex size-16 shrink-0 items-center justify-center rounded-2xl bg-[#d5e0f8] text-lg font-black text-[#434655] shadow-md">
            {getInitials(featured.contact_name)}
          </div>
          <div>
            <h3 className="mb-1 text-2xl font-bold text-[#0d1c2e]">
              {featured.contact_name}
            </h3>
            <p className="flex items-center text-sm font-medium text-[#434655]">
              {featured.company || "—"} ·{" "}
              {featured.channel === "telegram" ? "Telegram" : "Email"}
            </p>
          </div>
        </div>

        {/* AI Suggestion */}
        <div className="relative mb-8 rounded-xl bg-[#e1e0ff] p-6">
          <div className="absolute -top-3 left-6 flex items-center gap-1 rounded-md bg-[#3e3fcc] px-3 py-1 text-[0.6rem] font-bold text-white">
            <Sparkles className="size-3" />
            ИИ РЕКОМЕНДАЦИЯ
          </div>
          <p className="mb-2 text-lg font-bold text-[#2f2ebe]">
            &laquo;Напомните о диалоге, который начался{" "}
            {getTimeAgo(featured.created_at)} назад.&raquo;
          </p>
          <p className="text-sm italic text-[#2f2ebe]/70">
            {featured.first_message
              ? `Последнее сообщение: "${featured.first_message.slice(0, 120)}${featured.first_message.length > 120 ? "..." : ""}"`
              : "Floq рекомендует связаться с лидом для сохранения импульса."}
          </p>
        </div>

        {/* Actions */}
        <div className="flex items-center justify-between pt-4">
          <div className="flex gap-3">
            <button className="flex items-center gap-2 rounded-lg bg-gradient-to-r from-[#004ac6] to-[#2563eb] px-6 py-3 font-bold text-white shadow-md transition-all hover:opacity-90">
              <Send className="size-4" />
              Отправить напоминание
            </button>
            <button className="rounded-lg bg-[#eff4ff] px-6 py-3 font-bold text-[#434655] transition-all hover:bg-[#dce9ff]">
              Отложить
            </button>
          </div>
          <button className="flex items-center gap-1 text-sm font-bold text-[#434655] transition-colors hover:text-[#ba1a1a]">
            <X className="size-[18px]" />
            Закрыть
          </button>
        </div>
      </div>
    </div>
  );
}
