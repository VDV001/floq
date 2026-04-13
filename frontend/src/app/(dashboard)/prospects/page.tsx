"use client";

import { useState, useEffect } from "react";
import {
  Search,
  Upload,
  Download,
  ChevronLeft,
  ChevronRight,
  Sparkles,
  ShieldCheck,
  CheckCircle2,
  AlertTriangle,
  XCircle,
  MinusCircle,
  Globe,
  MoreHorizontal,
  ArrowRight,
  Mail,
  Phone,
  MessageCircle,
  MessageSquare,
} from "lucide-react";
import { api } from "@/lib/api";
import { SourceCombobox } from "@/components/ui/source-combobox";

/* ------------------------------------------------------------------ */
/*  Types & data                                                       */
/* ------------------------------------------------------------------ */

type ProspectStatus =
  | "Новый"
  | "В секвенции"
  | "Ответил"
  | "Конвертирован"
  | "Отписался";

interface UIProspect {
  initials: string;
  avatarColor: string;
  name: string;
  company: string;
  position: string;
  email: string;
  phone: string;
  whatsapp: string;
  telegramUsername: string;
  status: ProspectStatus;
  verifyStatus: "not_checked" | "valid" | "risky" | "invalid";
  verifyScore: number;
}

function mapProspectStatus(s: string): ProspectStatus {
  const m: Record<string, ProspectStatus> = {
    new: "Новый",
    in_sequence: "В секвенции",
    replied: "Ответил",
    converted: "Конвертирован",
    opted_out: "Отписался",
  };
  return m[s] || "Новый";
}

function mapProspects(data: { name: string; company: string; title: string; email: string; phone: string; whatsapp: string; telegram_username: string; status: string; verify_status: string; verify_score: number }[]): UIProspect[] {
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
    status: mapProspectStatus(p.status),
    verifyStatus: p.verify_status as UIProspect["verifyStatus"],
    verifyScore: p.verify_score,
  }));
}

const STATUS_STYLES: Record<ProspectStatus, string> = {
  Новый: "bg-blue-100 text-blue-700",
  "В секвенции": "bg-purple-100 text-purple-700",
  Ответил: "bg-green-100 text-green-700",
  Конвертирован: "bg-green-600 text-white",
  Отписался: "bg-slate-200 text-slate-600",
};

const VERIFY_STYLES: Record<
  string,
  { text: string; icon: typeof MinusCircle; label: string }
> = {
  not_checked: { text: "text-gray-500", icon: MinusCircle, label: "Не проверен" },
  valid: { text: "text-green-700", icon: CheckCircle2, label: "Валидный" },
  risky: { text: "text-yellow-700", icon: AlertTriangle, label: "Рискованный" },
  invalid: { text: "text-red-700", icon: XCircle, label: "Невалидный" },
};

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function ProspectsPage() {
  const [prospects, setProspects] = useState<UIProspect[]>([]);
  const [searchQuery, setSearchQuery] = useState("");
  const [loading, setLoading] = useState(false);
  const [formName, setFormName] = useState("");
  const [formCompany, setFormCompany] = useState("");
  const [formPosition, setFormPosition] = useState("");
  const [formEmail, setFormEmail] = useState("");
  const [formPhone, setFormPhone] = useState("");
  const [formTgUsername, setFormTgUsername] = useState("");
  const [formWhatsApp, setFormWhatsApp] = useState("");
  const [formSourceId, setFormSourceId] = useState<string | null>(null);
  const [scrapeUrl, setScrapeUrl] = useState("");
  const [scrapeLoading, setScrapeLoading] = useState(false);
  const [scrapeResults, setScrapeResults] = useState<string[]>([]);
  const [scrapeError, setScrapeError] = useState("");
  const [page, setPage] = useState(1);
  const perPage = 15;

  const fetchProspects = async () => {
    setLoading(true);
    try {
      const data = await api.getProspects();
      setProspects(mapProspects(data));
    } catch {
      // keep current list on error
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchProspects();
  }, []);

  const handleAddProspect = async () => {
    if (!formName) { alert("Введите имя"); return; }
    try {
      await api.createProspect({
        name: formName,
        company: formCompany,
        title: formPosition,
        email: formEmail,
        phone: formPhone || undefined,
        telegram_username: formTgUsername || undefined,
        whatsapp: formWhatsApp || undefined,
        source_id: formSourceId || undefined,
      });
      await fetchProspects();
      setFormName("");
      setFormCompany("");
      setFormPosition("");
      setFormEmail("");
      setFormPhone("");
      setFormTgUsername("");
      setFormWhatsApp("");
      setFormSourceId(null);
    } catch {
      alert("Ошибка добавления");
    }
  };

  const handleVerifyBatch = async () => {
    try {
      await api.verifyBatch();
      await fetchProspects();
    } catch {
      alert("Ошибка проверки");
    }
  };

  const filteredProspects = searchQuery
    ? prospects.filter((p) => {
        const q = searchQuery.toLowerCase();
        return (
          p.name.toLowerCase().includes(q) ||
          p.company.toLowerCase().includes(q) ||
          p.email.toLowerCase().includes(q) ||
          p.position.toLowerCase().includes(q)
        );
      })
    : prospects;

  const totalPages = Math.max(1, Math.ceil(filteredProspects.length / perPage));
  const safePage = Math.min(page, totalPages);
  const pagedProspects = filteredProspects.slice((safePage - 1) * perPage, safePage * perPage);
  const rangeStart = (safePage - 1) * perPage + 1;
  const rangeEnd = Math.min(safePage * perPage, filteredProspects.length);

  // Reset page on search change
  useEffect(() => { setPage(1); }, [searchQuery]);

  const handleScrape = async () => {
    setScrapeLoading(true);
    setScrapeError("");
    setScrapeResults([]);
    try {
      const res = await api.scrapeWebsite(scrapeUrl);
      setScrapeResults(res.emails);
      if (res.emails.length === 0) setScrapeError("Email не найдены на этом сайте");
    } catch {
      setScrapeError("Не удалось загрузить сайт");
    } finally {
      setScrapeLoading(false);
    }
  };

  return (
    <div className="min-h-full">
      {/* Top search bar */}
      <header className="flex h-16 items-center justify-between px-4 sm:px-6 lg:px-10">
        <div className="relative max-w-md flex-1">
          <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-slate-400" />
          <input
            type="text"
            placeholder="Поиск проспектов..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full rounded-full border-none bg-[#eff4ff] py-2 pl-10 pr-4 text-sm placeholder-slate-400 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
          />
        </div>
      </header>

      {/* Page content */}
      <section className="px-4 sm:px-6 lg:px-10 py-8">
        {/* Header */}
        <div className="mb-10 flex flex-col gap-6 md:flex-row md:items-end md:justify-between">
          <div>
            <h2 className="mb-2 text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">
              Проспекты
            </h2>
            <p className="font-medium text-[#434655]">
              {prospects.length} контактов{searchQuery ? `, найдено ${filteredProspects.length}` : ""}
            </p>
          </div>
          <div className="flex items-center gap-3">
            <button
              onClick={handleVerifyBatch}
              className="flex items-center gap-2 rounded-lg border border-[#c3c6d7]/30 bg-[#c3c6d7]/10 px-5 py-2.5 font-semibold text-[#0d1c2e] transition-all hover:bg-[#c3c6d7]/20"
            >
              <ShieldCheck className="size-5" />
              Проверить базу
            </button>
            <button
              onClick={() => api.exportProspectsCSV().catch(() => alert("Ошибка экспорта"))}
              className="flex items-center gap-2 rounded-lg border border-[#c3c6d7]/30 bg-[#c3c6d7]/10 px-5 py-2.5 font-semibold text-[#0d1c2e] transition-all hover:bg-[#c3c6d7]/20"
            >
              <Download className="size-5" />
              Экспорт CSV
            </button>
            <label className="flex cursor-pointer items-center gap-2 rounded-lg border border-[#c3c6d7]/30 bg-[#c3c6d7]/10 px-5 py-2.5 font-semibold text-[#0d1c2e] transition-all hover:bg-[#c3c6d7]/20">
              <Upload className="size-5" />
              Импорт CSV
              <input
                type="file"
                accept=".csv"
                className="hidden"
                onChange={async (e) => {
                  const file = e.target.files?.[0];
                  if (!file) return;
                  try {
                    const res = await api.importProspectsCSV(file);
                    alert(`Импортировано ${res.imported} проспектов`);
                    await fetchProspects();
                  } catch { alert("Ошибка импорта"); }
                  e.target.value = "";
                }}
              />
            </label>
          </div>
        </div>

        {/* Bento grid: table + sidebar */}
        <div className="grid grid-cols-12 gap-6">
          {/* Table (9 cols) */}
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
                    <th className="w-12 px-6 py-4">
                      <input
                        type="checkbox"
                        className="rounded border-[#c3c6d7] text-[#004ac6] focus:ring-[#004ac6]"
                      />
                    </th>
                    <th className="px-6 py-4 text-xs font-bold uppercase tracking-wider text-[#434655]">
                      Имя
                    </th>
                    <th className="px-6 py-4 text-xs font-bold uppercase tracking-wider text-[#434655]">
                      Компания / Должность
                    </th>
                    <th className="px-6 py-4 text-xs font-bold uppercase tracking-wider text-[#434655]">
                      Email
                    </th>
                    <th className="px-6 py-4 text-xs font-bold uppercase tracking-wider text-[#434655]">
                      Каналы
                    </th>
                    <th className="px-6 py-4 text-xs font-bold uppercase tracking-wider text-[#434655]">
                      Проверка
                    </th>
                    <th className="px-6 py-4 text-xs font-bold uppercase tracking-wider text-[#434655]">
                      Статус
                    </th>
                    <th className="px-6 py-4 text-xs font-bold uppercase tracking-wider text-[#434655]">
                      Действия
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-[#c3c6d7]/5">
                  {!loading && prospects.length === 0 && (
                    <tr>
                      <td colSpan={8} className="px-6 py-12 text-center text-sm text-[#434655]">
                        Нет проспектов
                      </td>
                    </tr>
                  )}
                  {pagedProspects.map((p, idx) => (
                    <tr
                      key={`${p.email || idx}-${idx}`}
                      className="transition-colors hover:bg-[#eff4ff]/30"
                    >
                      <td className="px-6 py-4">
                        <input
                          type="checkbox"
                          className="rounded border-[#c3c6d7] text-[#004ac6] focus:ring-[#004ac6]"
                        />
                      </td>
                      <td className="px-6 py-4">
                        <div className="flex items-center gap-3">
                          <div
                            className={`flex size-10 shrink-0 items-center justify-center rounded-lg font-bold ${p.avatarColor.includes("text-") ? p.avatarColor : p.avatarColor + " text-[#00174b]"}`}
                          >
                            {p.initials}
                          </div>
                          <span className="font-semibold text-[#0d1c2e]">
                            {p.name}
                          </span>
                        </div>
                      </td>
                      <td className="px-6 py-4">
                        <p className="font-medium text-[#0d1c2e]">{p.company}</p>
                        <p className="text-xs text-[#434655]">{p.position}</p>
                      </td>
                      <td className="px-6 py-4">
                        <span className="text-sm font-medium text-[#004ac6] underline underline-offset-4 decoration-[#004ac6]/20">
                          {p.email}
                        </span>
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
                        {(() => {
                          const vs = VERIFY_STYLES[p.verifyStatus];
                          const Icon = vs.icon;
                          return (
                            <div className="flex items-center gap-1.5">
                              <Icon className={`size-4 ${vs.text}`} />
                              <span className={`text-xs ${vs.text}`}>
                                {vs.label}
                                {p.verifyScore > 0 && ` (${p.verifyScore})`}
                              </span>
                            </div>
                          );
                        })()}
                      </td>
                      <td className="px-6 py-4">
                        <span
                          className={`whitespace-nowrap rounded-full px-3 py-1 text-[11px] font-bold uppercase tracking-wide ${STATUS_STYLES[p.status]}`}
                        >
                          {p.status}
                        </span>
                      </td>
                      <td className="px-6 py-4">
                        <button className="text-slate-400 transition-colors hover:text-[#0d1c2e]">
                          <MoreHorizontal className="size-5" />
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {/* Pagination */}
            <div className="flex items-center justify-between border-t border-[#c3c6d7]/10 bg-[#eff4ff]/30 px-6 py-4">
              <p className="text-xs font-medium text-[#434655]">
                {rangeStart}–{rangeEnd} из {filteredProspects.length} проспектов
              </p>
              {totalPages > 1 && (
                <div className="flex gap-2">
                  <button
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                    disabled={safePage <= 1}
                    className="flex size-8 items-center justify-center rounded border border-[#c3c6d7]/30 bg-white text-slate-400 shadow-sm transition-all hover:text-[#004ac6] disabled:opacity-40"
                  >
                    <ChevronLeft className="size-[18px]" />
                  </button>
                  {Array.from({ length: totalPages }, (_, i) => i + 1).map((p) => (
                    <button
                      key={p}
                      onClick={() => setPage(p)}
                      className={`flex size-8 items-center justify-center rounded text-xs font-bold shadow-sm transition-all ${
                        p === safePage
                          ? "bg-[#004ac6] text-white shadow-md"
                          : "border border-[#c3c6d7]/30 bg-white text-slate-600 hover:bg-slate-50"
                      }`}
                    >
                      {p}
                    </button>
                  ))}
                  <button
                    onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                    disabled={safePage >= totalPages}
                    className="flex size-8 items-center justify-center rounded border border-[#c3c6d7]/30 bg-white text-slate-400 shadow-sm transition-all hover:text-[#004ac6] disabled:opacity-40"
                  >
                    <ChevronRight className="size-[18px]" />
                  </button>
                </div>
              )}
            </div>
          </div>

          {/* Right sidebar (3 cols) */}
          <div className="col-span-12 space-y-6 lg:col-span-3">
            {/* New contact form */}
            <div id="add-form" className="rounded-xl border border-[#c3c6d7]/10 bg-white p-6 shadow-sm">
              <h3 className="mb-6 text-xl font-bold text-[#0d1c2e]">
                Новый контакт
              </h3>
              <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); handleAddProspect(); }}>
                <div>
                  <label className="mb-2 block text-xs font-bold uppercase tracking-wider text-[#434655]">
                    Имя
                  </label>
                  <input
                    className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-2.5 text-sm placeholder-slate-400 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
                    placeholder="Введите имя"
                    value={formName}
                    onChange={(e) => setFormName(e.target.value)}
                  />
                </div>
                <div>
                  <label className="mb-2 block text-xs font-bold uppercase tracking-wider text-[#434655]">
                    Компания
                  </label>
                  <input
                    className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-2.5 text-sm placeholder-slate-400 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
                    placeholder="Название компании"
                    value={formCompany}
                    onChange={(e) => setFormCompany(e.target.value)}
                  />
                </div>
                <div>
                  <label className="mb-2 block text-xs font-bold uppercase tracking-wider text-[#434655]">
                    Должность
                  </label>
                  <input
                    className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-2.5 text-sm placeholder-slate-400 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
                    placeholder="Напр. Head of Sales"
                    value={formPosition}
                    onChange={(e) => setFormPosition(e.target.value)}
                  />
                </div>
                <div>
                  <label className="mb-2 block text-xs font-bold uppercase tracking-wider text-[#434655]">
                    Email
                  </label>
                  <input
                    className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-2.5 text-sm placeholder-slate-400 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
                    placeholder="email@example.com"
                    type="email"
                    value={formEmail}
                    onChange={(e) => setFormEmail(e.target.value)}
                  />
                </div>
                <div>
                  <label className="mb-2 block text-xs font-bold uppercase tracking-wider text-[#434655]">
                    Телефон
                  </label>
                  <input
                    className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-2.5 text-sm placeholder-slate-400 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
                    placeholder="+7 900 123-45-67"
                    type="tel"
                    value={formPhone}
                    onChange={(e) => setFormPhone(e.target.value)}
                  />
                </div>
                <div>
                  <label className="mb-2 block text-xs font-bold uppercase tracking-wider text-[#434655]">
                    Telegram
                  </label>
                  <input
                    className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-2.5 text-sm placeholder-slate-400 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
                    placeholder="username (без @)"
                    value={formTgUsername}
                    onChange={(e) => setFormTgUsername(e.target.value.replace("@", ""))}
                  />
                </div>
                <div>
                  <label className="mb-2 block text-xs font-bold uppercase tracking-wider text-[#434655]">
                    WhatsApp
                  </label>
                  <input
                    className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-2.5 text-sm placeholder-slate-400 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
                    placeholder="+7 900 123-45-67"
                    value={formWhatsApp}
                    onChange={(e) => setFormWhatsApp(e.target.value)}
                  />
                </div>
                <div>
                  <label className="mb-2 block text-xs font-bold uppercase tracking-wider text-[#434655]">
                    Источник
                  </label>
                  <SourceCombobox value={formSourceId} onChange={setFormSourceId} />
                </div>
                <button
                  type="submit"
                  className="mt-4 w-full rounded-lg bg-[#004ac6] py-3 font-bold text-white shadow-lg shadow-[#004ac6]/20 transition-all hover:scale-[0.98]"
                >
                  Добавить
                </button>
              </form>
            </div>

            {/* Website scraper */}
            <div className="rounded-xl border border-[#c3c6d7]/10 bg-white p-6 shadow-sm">
              <h3 className="mb-4 flex items-center gap-2 text-sm font-bold text-[#0d1c2e]">
                <Globe className="size-4" />
                Поиск email по сайту
              </h3>
              <input
                className="mb-3 w-full rounded-lg border-none bg-[#eff4ff] px-4 py-2.5 text-sm placeholder-slate-400 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
                placeholder="https://company.ru"
                value={scrapeUrl}
                onChange={(e) => setScrapeUrl(e.target.value)}
              />
              <button
                className="w-full rounded-lg border border-[#c3c6d7]/30 py-2.5 text-sm font-semibold text-[#0d1c2e] transition-all hover:bg-[#eff4ff] disabled:opacity-50"
                onClick={handleScrape}
                disabled={scrapeLoading || !scrapeUrl}
              >
                {scrapeLoading ? "Поиск..." : "Найти email"}
              </button>
              {scrapeResults.length > 0 && (
                <div className="mt-3 space-y-1">
                  <p className="text-xs font-medium text-[#434655]">
                    Найдено: {scrapeResults.length}
                  </p>
                  {scrapeResults.map((email) => (
                    <div
                      key={email}
                      className="rounded-lg bg-[#eff4ff] px-3 py-1.5 text-sm text-[#0d1c2e]"
                    >
                      {email}
                    </div>
                  ))}
                </div>
              )}
              {scrapeError && (
                <p className="mt-2 text-xs text-[#ba1a1a]">{scrapeError}</p>
              )}
            </div>

            {/* AI Insight */}
            <div className="group relative overflow-hidden rounded-xl border border-[#585be6]/10 bg-[#e1e0ff] p-6 shadow-sm">
              <div className="absolute right-0 top-0 p-4 opacity-10 transition-opacity group-hover:opacity-20">
                <Sparkles className="size-16" />
              </div>
              <h4 className="mb-3 flex items-center gap-2 text-sm font-bold text-[#07006c]">
                <Sparkles className="size-[18px]" />
                AI Аналитика
              </h4>
              <p className="text-xs font-medium leading-relaxed text-[#2f2ebe]">
                {prospects.length === 0
                  ? "Добавьте первого проспекта через форму или импорт CSV чтобы начать работу с базой."
                  : `${prospects.filter(p => p.verifyStatus === "not_checked").length} проспектов не проверены. Нажмите «Проверить базу» для верификации email.`}
              </p>
              <button className="mt-4 flex items-center gap-1 text-xs font-bold text-[#3e3fcc] hover:underline">
                Посмотреть список
                <ArrowRight className="size-3.5" />
              </button>
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}
