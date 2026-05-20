"use client";

import { api } from "@/lib/api";
import { Upload, Download } from "lucide-react";
import { cn } from "@/lib/utils";
import { FILTER_TABS } from "@/components/inbox/constants";
import { PipelineSidebar } from "@/components/inbox/PipelineSidebar";
import { LeadCard } from "@/components/leads/LeadCard";
import { PendingQueueTabs } from "@/components/pending-queue/PendingQueueTabs";
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
          <PendingQueueTabs />
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
              <LeadCard
                key={lead.id}
                id={lead.id}
                company={lead.company}
                contact={lead.contact}
                channel={lead.channel}
                preview={lead.preview}
                timeAgo={lead.timeAgo}
                status={lead.status}
                sourceName={lead.sourceName}
                pendingRepliesCount={lead.pendingRepliesCount}
                suggestionCount={suggestionCounts[lead.id]}
              />
            ))}
          </div>
        </div>
      </section>
    </div>
  );
}
