"use client";

import { Search, Upload, Download, ShieldCheck, Sparkles, ArrowRight, CheckCircle2, AlertTriangle, XCircle } from "lucide-react";
import { api } from "@/lib/api";
import { ProspectTable } from "@/components/prospects/ProspectTable";
import { AddProspectForm } from "@/components/prospects/AddProspectForm";
import { WebsiteScraper } from "@/components/prospects/WebsiteScraper";
import { SourceAnalytics } from "@/components/prospects/SourceAnalytics";
import { useProspectsPage } from "@/hooks/useProspectsPage";

export default function ProspectsPage() {
  const {
    prospects, searchQuery, setSearchQuery, sourceFilter, setSourceFilter,
    loading, verifying, toast, setToast, sourceStats, page, setPage,
    fetchProspects, handleVerifyBatch, sourceNames, filteredProspects,
    totalPages, pagedProspects, rangeStart, rangeEnd,
  } = useProspectsPage();

  return (
    <div className="min-h-full relative">
      {toast && (
        <div className={`fixed top-6 right-6 z-50 flex items-center gap-3 rounded-xl px-5 py-3.5 shadow-lg transition-all animate-in fade-in slide-in-from-top-2 ${
          toast.type === "success" ? "bg-[#0d1c2e] text-white" : "bg-[#ffdad6] text-[#93000a]"
        }`}>
          {toast.type === "success" ? <CheckCircle2 className="size-5 text-emerald-400" /> : <AlertTriangle className="size-5" />}
          <span className="text-sm font-semibold">{toast.message}</span>
          <button onClick={() => setToast(null)} className="ml-2 opacity-60 hover:opacity-100"><XCircle className="size-4" /></button>
        </div>
      )}

      <header className="flex h-16 items-center gap-3 px-4 sm:px-6 lg:px-10">
        <div className="relative max-w-md flex-1">
          <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-slate-400" />
          <input type="text" placeholder="Поиск проспектов..." value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full rounded-full border-none bg-[#eff4ff] py-2 pl-10 pr-4 text-sm placeholder-slate-400 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20" />
        </div>
        {sourceNames.length > 0 && (
          <select value={sourceFilter} onChange={(e) => setSourceFilter(e.target.value)}
            className="rounded-lg border-none bg-[#eff4ff] px-3 py-2 text-sm text-[#0d1c2e] outline-none focus:ring-2 focus:ring-[#004ac6]/20">
            <option value="">Все источники</option>
            {sourceNames.map((s) => <option key={s} value={s}>{s}</option>)}
          </select>
        )}
      </header>

      <section className="px-4 sm:px-6 lg:px-10 py-8">
        <div className="mb-10 flex flex-col gap-6 md:flex-row md:items-end md:justify-between">
          <div>
            <h2 className="mb-2 text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">Проспекты</h2>
            <p className="font-medium text-[#434655]">{prospects.length} контактов{searchQuery ? `, найдено ${filteredProspects.length}` : ""}</p>
          </div>
          <div className="flex items-center gap-3">
            <button onClick={handleVerifyBatch} disabled={verifying}
              className="flex items-center gap-2 rounded-lg border border-[#c3c6d7]/30 bg-[#c3c6d7]/10 px-5 py-2.5 font-semibold text-[#0d1c2e] transition-all hover:bg-[#c3c6d7]/20 disabled:opacity-60">
              {verifying ? <div className="size-5 animate-spin rounded-full border-2 border-[#004ac6] border-t-transparent" /> : <ShieldCheck className="size-5" />}
              {verifying ? "Проверяю..." : "Проверить базу"}
            </button>
            <button onClick={() => api.exportProspectsCSV().catch(() => alert("Ошибка экспорта"))}
              className="flex items-center gap-2 rounded-lg border border-[#c3c6d7]/30 bg-[#c3c6d7]/10 px-5 py-2.5 font-semibold text-[#0d1c2e] transition-all hover:bg-[#c3c6d7]/20">
              <Download className="size-5" /> Экспорт CSV
            </button>
            <label className="flex cursor-pointer items-center gap-2 rounded-lg border border-[#c3c6d7]/30 bg-[#c3c6d7]/10 px-5 py-2.5 font-semibold text-[#0d1c2e] transition-all hover:bg-[#c3c6d7]/20">
              <Upload className="size-5" /> Импорт CSV
              <input type="file" accept=".csv" className="hidden" onChange={async (e) => {
                const file = e.target.files?.[0];
                if (!file) return;
                try { const res = await api.importProspectsCSV(file); alert(`Импортировано ${res.imported} проспектов`); await fetchProspects(); }
                catch { alert("Ошибка импорта"); }
                e.target.value = "";
              }} />
            </label>
          </div>
        </div>

        <div className="grid grid-cols-12 gap-6">
          <ProspectTable
            prospects={pagedProspects} loading={loading} totalCount={filteredProspects.length}
            page={page} totalPages={totalPages} rangeStart={rangeStart} rangeEnd={rangeEnd}
            onPageChange={setPage}
          />

          <div className="col-span-12 space-y-6 lg:col-span-3">
            <AddProspectForm onAdded={fetchProspects} />
            <WebsiteScraper />

            <div className="group relative overflow-hidden rounded-xl border border-[#585be6]/10 bg-[#e1e0ff] p-6 shadow-sm">
              <div className="absolute right-0 top-0 p-4 opacity-10 transition-opacity group-hover:opacity-20"><Sparkles className="size-16" /></div>
              <h4 className="mb-3 flex items-center gap-2 text-sm font-bold text-[#07006c]"><Sparkles className="size-[18px]" /> AI Аналитика</h4>
              <p className="text-xs font-medium leading-relaxed text-[#2f2ebe]">
                {prospects.length === 0
                  ? "Добавьте первого проспекта через форму или импорт CSV чтобы начать работу с базой."
                  : `${prospects.filter(p => p.verifyStatus === "not_checked").length} проспектов не проверены. Нажмите «Проверить базу» для верификации email.`}
              </p>
              <button className="mt-4 flex items-center gap-1 text-xs font-bold text-[#3e3fcc] hover:underline">Посмотреть список <ArrowRight className="size-3.5" /></button>
            </div>

            <SourceAnalytics stats={sourceStats} />
          </div>
        </div>
      </section>
    </div>
  );
}
