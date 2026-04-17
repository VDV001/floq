import { ChevronLeft, ChevronRight, MoreHorizontal, Mail, Phone, MessageCircle, MessageSquare } from "lucide-react";
import type { UIProspect } from "./constants";
import { STATUS_STYLES, VERIFY_STYLES } from "./constants";

interface ProspectTableProps {
  prospects: UIProspect[];
  loading: boolean;
  totalCount: number;
  page: number;
  totalPages: number;
  rangeStart: number;
  rangeEnd: number;
  onPageChange: (page: number) => void;
}

export function ProspectTable({ prospects, loading, totalCount, page, totalPages, rangeStart, rangeEnd, onPageChange }: ProspectTableProps) {
  return (
    <div className="col-span-12 overflow-hidden rounded-xl border border-[#c3c6d7]/10 bg-white shadow-sm lg:col-span-9">
      {loading && (
        <div className="flex items-center justify-center py-8">
          <div className="size-6 animate-spin rounded-full border-2 border-[#004ac6] border-t-transparent" />
        </div>
      )}
      <div className="overflow-x-auto">
        <table className="w-full border-collapse text-left">
          <thead>
            <tr className="bg-[#eff4ff]/50">
              <th className="w-12 px-6 py-4"><input type="checkbox" className="rounded border-[#c3c6d7] text-[#004ac6] focus:ring-[#004ac6]" /></th>
              <th className="px-6 py-4 text-xs font-bold uppercase tracking-wider text-[#434655]">Имя</th>
              <th className="px-6 py-4 text-xs font-bold uppercase tracking-wider text-[#434655]">Компания / Должность</th>
              <th className="px-6 py-4 text-xs font-bold uppercase tracking-wider text-[#434655]">Email</th>
              <th className="px-6 py-4 text-xs font-bold uppercase tracking-wider text-[#434655]">Каналы</th>
              <th className="px-6 py-4 text-xs font-bold uppercase tracking-wider text-[#434655]">Проверка</th>
              <th className="px-6 py-4 text-xs font-bold uppercase tracking-wider text-[#434655]">Статус</th>
              <th className="px-6 py-4 text-xs font-bold uppercase tracking-wider text-[#434655]">Действия</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[#c3c6d7]/5">
            {!loading && totalCount === 0 && (
              <tr><td colSpan={8} className="px-6 py-12 text-center text-sm text-[#434655]">Нет проспектов</td></tr>
            )}
            {prospects.map((p, idx) => {
              const vs = VERIFY_STYLES[p.verifyStatus];
              const Icon = vs.icon;
              return (
                <tr key={`${p.email || idx}-${idx}`} className="transition-colors hover:bg-[#eff4ff]/30">
                  <td className="px-6 py-4"><input type="checkbox" className="rounded border-[#c3c6d7] text-[#004ac6] focus:ring-[#004ac6]" /></td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-3">
                      <div className={`flex size-10 shrink-0 items-center justify-center rounded-lg font-bold ${p.avatarColor} text-[#00174b]`}>{p.initials}</div>
                      <span className="font-semibold text-[#0d1c2e]">{p.name}</span>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <p className="font-medium text-[#0d1c2e]">{p.company}</p>
                    <p className="text-xs text-[#434655]">{p.position}</p>
                  </td>
                  <td className="px-6 py-4">
                    <span className="text-sm font-medium text-[#004ac6] underline underline-offset-4 decoration-[#004ac6]/20">{p.email}</span>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-1.5">
                      <span title={p.email ? `Email: ${p.email}` : "Email не указан"}><Mail className={`size-4 ${p.email ? "text-blue-600" : "text-slate-300"}`} /></span>
                      <span title={p.phone ? `Тел: ${p.phone}` : "Телефон не указан"}><Phone className={`size-4 ${p.phone ? "text-green-600" : "text-slate-300"}`} /></span>
                      <span title={p.telegramUsername ? `TG: @${p.telegramUsername}` : "Telegram не указан"}><MessageCircle className={`size-4 ${p.telegramUsername ? "text-sky-500" : "text-slate-300"}`} /></span>
                      <span title={p.whatsapp ? `WA: ${p.whatsapp}` : "WhatsApp не указан"}><MessageSquare className={`size-4 ${p.whatsapp ? "text-emerald-500" : "text-slate-300"}`} /></span>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center gap-1.5">
                      <Icon className={`size-4 ${vs.text}`} />
                      <span className={`text-xs ${vs.text}`}>{vs.label}{p.verifyScore > 0 && ` (${p.verifyScore})`}</span>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <span className={`whitespace-nowrap rounded-full px-3 py-1 text-[11px] font-bold uppercase tracking-wide ${STATUS_STYLES[p.status]}`}>{p.status}</span>
                  </td>
                  <td className="px-6 py-4">
                    <button className="text-slate-400 transition-colors hover:text-[#0d1c2e]"><MoreHorizontal className="size-5" /></button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <div className="flex items-center justify-between border-t border-[#c3c6d7]/10 bg-[#eff4ff]/30 px-6 py-4">
        <p className="text-xs font-medium text-[#434655]">{rangeStart}–{rangeEnd} из {totalCount} проспектов</p>
        {totalPages > 1 && (
          <div className="flex gap-2">
            <button onClick={() => onPageChange(Math.max(1, page - 1))} disabled={page <= 1}
              className="flex size-8 items-center justify-center rounded border border-[#c3c6d7]/30 bg-white text-slate-400 shadow-sm transition-all hover:text-[#004ac6] disabled:opacity-40">
              <ChevronLeft className="size-[18px]" />
            </button>
            {Array.from({ length: totalPages }, (_, i) => i + 1).map((p) => (
              <button key={p} onClick={() => onPageChange(p)}
                className={`flex size-8 items-center justify-center rounded text-xs font-bold shadow-sm transition-all ${
                  p === page ? "bg-[#004ac6] text-white shadow-md" : "border border-[#c3c6d7]/30 bg-white text-slate-600 hover:bg-slate-50"
                }`}>{p}</button>
            ))}
            <button onClick={() => onPageChange(Math.min(totalPages, page + 1))} disabled={page >= totalPages}
              className="flex size-8 items-center justify-center rounded border border-[#c3c6d7]/30 bg-white text-slate-400 shadow-sm transition-all hover:text-[#004ac6] disabled:opacity-40">
              <ChevronRight className="size-[18px]" />
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
