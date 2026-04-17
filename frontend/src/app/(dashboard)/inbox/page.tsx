"use client";

import { api } from "@/lib/api";
import Link from "next/link";
import { Mail, Send, Upload, Download, Link2 } from "lucide-react";
import { cn } from "@/lib/utils";
import { STATUS_STYLES, FILTER_TABS } from "@/components/inbox/constants";
import { PipelineSidebar } from "@/components/inbox/PipelineSidebar";
import { useInboxPage } from "@/hooks/useInboxPage";

export default function InboxPage() {
  const {
    activeFilter, setActiveFilter, activeStage, setActiveStage,
    loading, leads, statusCounts, sourceFilter, setSourceFilter,
    suggestionCounts, filteredLeads,
  } = useInboxPage();

  return (
    <div className="flex h-full">
      <PipelineSidebar activeStage={activeStage} setActiveStage={setActiveStage}
        statusCounts={statusCounts} leads={leads} sourceFilter={sourceFilter} setSourceFilter={setSourceFilter} />

      <section className="flex-1 overflow-y-auto px-4 sm:px-8 lg:px-12 py-8">
        <div className="mx-auto max-w-4xl space-y-8">
          <div className="flex items-end justify-between">
            <div>
              <div className="flex items-center gap-3">
                <h2 className="text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">Лента лидов</h2>
                {loading && <div className="size-5 animate-spin rounded-full border-2 border-[#3b6ef6] border-t-transparent" />}
              </div>
              <p className="mt-1 text-sm text-[#434655]">Показано {leads.length} активных лидов для <span className="font-bold">Новые лиды</span></p>
            </div>
            <div className="flex items-center gap-2 mr-4">
              <button onClick={() => api.exportLeadsCSV().catch(() => alert("Ошибка экспорта"))}
                className="flex items-center gap-1.5 rounded-lg border border-[#c3c6d7]/30 bg-[#c3c6d7]/10 px-4 py-2 text-xs font-semibold text-[#0d1c2e] transition-all hover:bg-[#c3c6d7]/20">
                <Download className="size-4" /> Экспорт
              </button>
              <label className="flex cursor-pointer items-center gap-1.5 rounded-lg border border-[#c3c6d7]/30 bg-[#c3c6d7]/10 px-4 py-2 text-xs font-semibold text-[#0d1c2e] transition-all hover:bg-[#c3c6d7]/20">
                <Upload className="size-4" /> Импорт
                <input type="file" accept=".csv" className="hidden" onChange={async (e) => {
                  const file = e.target.files?.[0]; if (!file) return;
                  try { const res = await api.importLeadsCSV(file); alert(`Импортировано ${res.imported} лидов`); window.location.reload(); }
                  catch { alert("Ошибка импорта"); } e.target.value = "";
                }} />
              </label>
            </div>
            <div className="flex items-center gap-1 rounded-lg bg-[#eff4ff] p-1">
              {FILTER_TABS.map((tab) => (
                <button key={tab} onClick={() => setActiveFilter(tab)}
                  className={cn("rounded-md px-4 py-1.5 text-xs font-medium transition-colors",
                    activeFilter === tab ? "bg-white font-bold text-[#004ac6] shadow-sm" : "text-[#434655] hover:bg-[#dce9ff]")}>
                  {tab}
                </button>
              ))}
            </div>
          </div>

          <div className="space-y-3">
            {!loading && filteredLeads.length === 0 && (
              <div className="rounded-xl bg-white p-12 text-center">
                <p className="text-lg font-bold text-[#0d1c2e]">Нет лидов</p>
                <p className="mt-2 text-sm text-[#434655]">Напишите вашему Telegram боту чтобы создать первый лид</p>
              </div>
            )}
            {filteredLeads.map((lead) => (
              <Link key={lead.id} href={`/inbox/${lead.id}`}
                className="group relative flex cursor-pointer rounded-xl border border-transparent bg-white p-5 transition-all hover:border-[#c3c6d7]/10 hover:bg-[#dce9ff]/40">
                <div className="flex items-start gap-4 flex-1 min-w-0">
                  <div className={cn("flex size-12 shrink-0 items-center justify-center rounded-xl", lead.channel === "email" ? "bg-[#dbe1ff]" : "bg-[#d5e0f8]")}>
                    {lead.channel === "email" ? <Mail className="size-5 text-[#004ac6]" /> : <Send className="size-5 text-[#229ED9]" />}
                  </div>
                  <div className="min-w-0 flex-1">
                    <h4 className="font-bold leading-none text-[#0d1c2e]">{lead.company}</h4>
                    <p className="mt-1 text-xs font-medium text-[#737686]">{lead.channel === "email" ? "по email" : "через Telegram"} · {lead.contact}</p>
                    <div className="mt-2 flex items-center gap-2">
                      {lead.sourceName && <span className="rounded-full bg-[#eff4ff] px-2 py-0.5 text-[10px] font-semibold text-[#004ac6]">{lead.sourceName}</span>}
                    </div>
                    <p className="mt-1 line-clamp-2 text-sm leading-relaxed text-[#434655]">{lead.preview}</p>
                  </div>
                </div>
                <div className="ml-4 flex shrink-0 flex-col items-end gap-2">
                  <span className="text-[10px] font-bold uppercase tracking-wider text-[#737686]">{lead.timeAgo}</span>
                  <span className={cn("whitespace-nowrap rounded-full px-3 py-1 text-[10px] font-bold", STATUS_STYLES[lead.status])}>{lead.status}</span>
                  {suggestionCounts[lead.id] > 0 && (
                    <span className="inline-flex items-center gap-1 rounded-full bg-[#fff3cd] px-2 py-0.5 text-[10px] font-semibold text-[#8a5a00]"
                      title={`${suggestionCounts[lead.id]} возможных совпадений с проспектом`}>
                      <Link2 className="size-3" />{suggestionCounts[lead.id]}
                    </span>
                  )}
                </div>
              </Link>
            ))}
          </div>
        </div>
      </section>
    </div>
  );
}
