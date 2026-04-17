"use client";

import { Search, Send, CheckCheck, Bot } from "lucide-react";
import { Switch } from "@/components/ui/switch";
import { useOutbound } from "@/hooks/useOutbound";
import { formatTime } from "@/components/outbound/constants";
import { FilterPill } from "@/components/outbound/FilterPill";
import { MessageCard } from "@/components/outbound/MessageCard";

export default function OutboundPage() {
  const o = useOutbound();

  return (
    <div className="min-h-full">
      <header className="flex h-16 items-center justify-between px-4 sm:px-6 lg:px-10">
        <div className="flex items-center gap-3">
          <div className="relative w-96">
            <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-slate-400" />
            <input type="text" placeholder="Поиск по очереди..." value={o.search} onChange={(e) => o.setSearch(e.target.value)}
              className="w-full rounded-full border-none bg-[#eff4ff] py-2 pl-10 pr-4 text-sm placeholder-slate-400 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20" />
          </div>
          {o.tab === "queue" && o.messages.length > 0 && (
            <button onClick={o.handleApproveAll} disabled={o.approvingAll}
              className="flex items-center gap-1.5 rounded-full border border-[#004ac6] px-4 py-2 text-xs font-bold text-[#004ac6] transition-all hover:bg-[#004ac6] hover:text-white disabled:opacity-50">
              <CheckCheck className="size-3.5" />
              {o.approvingAll ? "Подтверждение..." : "Подтвердить все"}
            </button>
          )}
        </div>
      </header>

      <div className="mx-auto max-w-6xl px-4 sm:px-6 lg:px-10 py-8">
        <div className="mb-6">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="mb-1 text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">Очередь отправки</h2>
              <p className="text-sm font-medium text-[#434655]">Контроль качества AI-сообщений перед отправкой</p>
            </div>
            <span className="text-[11px] text-[#737686]">Обновлено: {formatTime(o.lastUpdated)}</span>
          </div>
        </div>

        {/* Stats */}
        <div className="mb-6 grid grid-cols-3 gap-3 sm:grid-cols-6">
          {[
            { label: "Отправлено", value: o.stats.sent, color: "text-[#0d1c2e]" },
            { label: "Одобрено", value: o.stats.approved, color: "text-[#0d1c2e]" },
            { label: "В очереди", value: o.stats.draft, color: "text-[#004ac6]" },
            { label: "Открыто", value: o.stats.opened ?? 0, color: "text-green-600" },
            { label: "Ответили", value: o.stats.replied ?? 0, color: "text-green-600" },
            { label: "Bounce", value: o.stats.bounced ?? 0, color: "text-red-500" },
          ].map((s) => (
            <div key={s.label} className="rounded-xl bg-white p-4 text-center shadow-sm ring-1 ring-[#c3c6d7]/10">
              <p className={`text-2xl font-black ${s.color}`}>{s.value}</p>
              <p className="mt-1 text-[10px] font-bold uppercase tracking-widest text-[#737686]">{s.label}</p>
            </div>
          ))}
        </div>

        {/* Tabs */}
        <div className="mb-4 flex gap-1 rounded-xl bg-[#eff4ff] p-1">
          <button onClick={() => { o.setTab("queue"); o.setStatusFilter("all"); }}
            className={`flex-1 rounded-lg px-4 py-2.5 text-sm font-bold transition-all ${o.tab === "queue" ? "bg-white text-[#0d1c2e] shadow-sm" : "text-[#434655] hover:text-[#0d1c2e]"}`}>
            В очереди{o.messages.length > 0 && ` (${o.messages.length})`}
          </button>
          <button onClick={() => o.setTab("sent")}
            className={`flex-1 rounded-lg px-4 py-2.5 text-sm font-bold transition-all ${o.tab === "sent" ? "bg-white text-[#0d1c2e] shadow-sm" : "text-[#434655] hover:text-[#0d1c2e]"}`}>
            Отправленные{o.sentMessages.length > 0 && ` (${o.sentMessages.length})`}
          </button>
        </div>

        {/* Filters */}
        <div className="mb-8 flex flex-wrap items-center gap-2">
          {o.tab === "queue" ? (
            <>
              <FilterPill label="Все" value="all" current={o.channelFilter} onChange={o.setChannelFilter} />
              <FilterPill label="Email" value="email" current={o.channelFilter} onChange={o.setChannelFilter} />
              <FilterPill label="Telegram" value="telegram" current={o.channelFilter} onChange={o.setChannelFilter} />
              <FilterPill label="Звонок" value="phone_call" current={o.channelFilter} onChange={o.setChannelFilter} />
            </>
          ) : (
            <>
              <FilterPill label="Все" value="all" current={o.statusFilter} onChange={o.setStatusFilter} />
              <FilterPill label="Отправлено" value="sent" current={o.statusFilter} onChange={o.setStatusFilter} />
              <FilterPill label="Одобрено" value="approved" current={o.statusFilter} onChange={o.setStatusFilter} />
              <FilterPill label="Отклонено" value="rejected" current={o.statusFilter} onChange={o.setStatusFilter} />
            </>
          )}
        </div>

        {/* Autopilot */}
        {o.tab === "queue" && (
          <div className="mb-10 flex items-center justify-between rounded-2xl border border-transparent bg-[#eff4ff] p-6 transition-all hover:border-[#c3c6d7]/20 hover:bg-white">
            <div className="flex items-center gap-4">
              <div className="flex size-12 items-center justify-center rounded-full bg-[#e1e0ff] text-[#3e3fcc]"><Bot className="size-6" /></div>
              <div>
                <h3 className="font-bold text-[#0d1c2e]">Автопилот</h3>
                <p className="text-sm text-[#434655]">Сообщения будут отправляться автоматически без вашего одобрения</p>
              </div>
            </div>
            <div className="flex items-center gap-4">
              <span className="text-xs font-bold uppercase tracking-wider text-[#737686]">{o.autopilot ? "Вкл" : "Выкл"}</span>
              <Switch checked={o.autopilot} onCheckedChange={o.setAutopilot} />
            </div>
          </div>
        )}

        {/* Messages */}
        <div className="space-y-4">
          {!o.loading && o.paginatedItems.length === 0 && (
            <div className="flex flex-col items-center justify-center rounded-2xl bg-white py-16 text-center">
              <Send className="mb-4 size-10 text-[#c3c6d7]" />
              <p className="text-lg font-semibold text-[#434655]">{o.tab === "queue" ? "Нет сообщений в очереди" : "Нет отправленных сообщений"}</p>
              <p className="mt-1 text-sm text-[#737686]">{o.tab === "queue" ? "Новые сообщения появятся здесь после генерации AI" : "Одобренные и отправленные сообщения будут отображаться здесь"}</p>
            </div>
          )}
          {o.paginatedItems.map((msg) => (
            <MessageCard key={msg.id} msg={msg} isQueue={o.tab === "queue"}
              onApprove={o.handleApprove} onReject={o.handleReject} onEdited={o.handleEdited} />
          ))}
        </div>

        {/* Pagination */}
        {o.filtered.length > o.ITEMS_PER_PAGE && (
          <div className="mt-8 flex items-center justify-center gap-4">
            <button onClick={() => o.setPage((p: number) => Math.max(1, p - 1))} disabled={o.safePage <= 1}
              className="rounded-lg border border-[#c3c6d7] px-4 py-2 text-sm font-bold text-[#434655] transition-colors hover:bg-[#eff4ff] disabled:opacity-40 disabled:hover:bg-transparent">
              &larr; Назад
            </button>
            <span className="text-sm font-medium text-[#737686]">
              {(o.safePage - 1) * o.ITEMS_PER_PAGE + 1}–{Math.min(o.safePage * o.ITEMS_PER_PAGE, o.filtered.length)} из {o.filtered.length}
            </span>
            <button onClick={() => o.setPage((p: number) => Math.min(o.totalPages, p + 1))} disabled={o.safePage >= o.totalPages}
              className="rounded-lg border border-[#c3c6d7] px-4 py-2 text-sm font-bold text-[#434655] transition-colors hover:bg-[#eff4ff] disabled:opacity-40 disabled:hover:bg-transparent">
              Далее &rarr;
            </button>
          </div>
        )}

        <footer className="mt-20 flex flex-col items-center justify-between gap-4 border-t border-[#c3c6d7]/10 py-10 md:flex-row">
          <p className="text-sm font-medium text-[#737686]">Создано Floq AI Sales Engine</p>
          <div className="flex gap-6 text-xs font-bold uppercase tracking-widest text-[#c3c6d7]">
            <a className="transition-colors hover:text-[#004ac6]" href="#">Политика</a>
            <a className="transition-colors hover:text-[#004ac6]" href="#">Условия</a>
            <a className="transition-colors hover:text-[#004ac6]" href="#">API</a>
          </div>
        </footer>
      </div>
    </div>
  );
}
