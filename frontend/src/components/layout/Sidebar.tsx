"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useState, useEffect } from "react";
import {
  Inbox,
  Users,
  GitBranch,
  Zap,
  Settings,
  Sparkles,
  LogOut,
  HelpCircle,
  UserPlus,
  Send,
  GraduationCap,
  Menu,
  X,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { api } from "@/lib/api";

const NAV_ITEMS = [
  { label: "Входящие", href: "/inbox", icon: Inbox, hint: "Все входящие сообщения из Telegram и Email в одном месте" },
  { label: "Лиды", href: "/alerts", icon: Users, hint: "Потенциальные клиенты, которые написали вам первыми. Здесь AI квалифицирует и оценивает каждый контакт" },
  { label: "Воронка", href: "/pipeline", icon: GitBranch, hint: "Визуальный путь лида: Новый → Квалифицирован → В диалоге → Фоллоуап → Закрыт" },
  { label: "Автоматизации", href: "/automations", icon: Zap, hint: "Автоматические действия: AI-квалификация, генерация ответов, фоллоуапы без ручной работы" },
  { label: "Проспекты", href: "/prospects", icon: UserPlus, hint: "База контактов для холодного аутрича. Импорт из CSV, парсинг 2GIS, ручное добавление" },
  { label: "Секвенции", href: "/sequences", icon: GitBranch, hint: "Цепочки автоматических касаний: Email → Telegram → Прозвон. AI пишет текст под каждый канал" },
  { label: "Очередь отправки", href: "/outbound", icon: Send, hint: "Сообщения, сгенерированные AI и ожидающие вашего одобрения перед отправкой" },
  { label: "Настройки", href: "/settings", icon: Settings, hint: "Подключение каналов (Telegram, Email), выбор AI-провайдера, уведомления" },
  { label: "Обучение", href: "/onboarding", icon: GraduationCap, hint: "Пошаговая настройка системы и полезные советы для начала работы" },
];

const PLAN_LABELS: Record<string, string> = {
  starter: "Starter",
  growth: "Growth",
  pro: "Pro",
};

export function Sidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const [mobileOpen, setMobileOpen] = useState(false);

  const [usage, setUsage] = useState<{
    plan: string;
    limit: number;
    month_leads: number;
    total_leads: number;
  } | null>(null);

  useEffect(() => {
    api.getUsage().then(setUsage).catch(() => {});
    // Refresh every 60 seconds
    const interval = setInterval(() => {
      api.getUsage().then(setUsage).catch(() => {});
    }, 60_000);
    return () => clearInterval(interval);
  }, []);

  // Close mobile sidebar on route change
  useEffect(() => {
    setMobileOpen(false);
  }, [pathname]);

  // Close on Escape key
  useEffect(() => {
    if (!mobileOpen) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") setMobileOpen(false);
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [mobileOpen]);

  const handleLogout = () => {
    localStorage.removeItem("token");
    localStorage.removeItem("refresh_token");
    router.replace("/login");
  };

  const usedPercent = usage
    ? Math.min((usage.month_leads / usage.limit) * 100, 100)
    : 0;
  const isNearLimit = usage ? usage.month_leads >= usage.limit * 0.8 : false;
  const isOverLimit = usage ? usage.month_leads >= usage.limit : false;

  const sidebarContent = (
    <>
      {/* Logo */}
      <div className="px-6 pt-8 pb-6">
        <div className="flex items-center gap-3">
          <Sparkles className="size-8 text-[#3b6ef6]" />
          <div>
            <h1 className="text-2xl font-bold tracking-tight text-[#0d1c2e]">
              Floq
            </h1>
            <p className="text-[10px] font-bold uppercase tracking-widest text-[#434655]/60">
              AI Sales Assistant
            </p>
          </div>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 space-y-1 px-4">
        {NAV_ITEMS.map((item) => {
          const isActive = pathname.startsWith(item.href);
          return (
            <div key={item.href} className="group/row relative flex items-center">
              <Link
                href={item.href}
                className={cn(
                  "flex flex-1 items-center gap-3 rounded-lg px-4 py-3 text-sm font-medium transition-colors duration-200",
                  isActive
                    ? "bg-white/50 font-bold text-[#2563eb]"
                    : "text-[#434655] hover:bg-[#dce9ff]"
                )}
              >
                <item.icon className="size-5" />
                {item.label}
              </Link>
              {/* Info dot — visible on row hover, tooltip on dot hover */}
              <span className="group/dot absolute right-3 flex size-4 cursor-default items-center justify-center rounded-full text-[9px] font-bold text-[#434655]/0 transition-colors duration-200 group-hover/row:text-[#434655]/30">
                ?
                <span className="pointer-events-none absolute left-full top-1/2 z-50 ml-2 w-48 -translate-y-1/2 rounded-lg bg-[#0d1c2e] px-3 py-2.5 text-[11px] font-normal leading-relaxed text-white/90 opacity-0 shadow-xl transition-opacity duration-150 group-hover/dot:pointer-events-auto group-hover/dot:opacity-100">
                  {item.hint}
                  <span className="absolute right-full top-1/2 -translate-y-1/2 border-4 border-transparent border-r-[#0d1c2e]" />
                </span>
              </span>
            </div>
          );
        })}
      </nav>

      {/* Bottom section */}
      <div className="mt-auto space-y-4 px-4 pb-8">
        {/* Usage */}
        <div className="rounded-xl bg-[#dbe1ff]/30 p-4">
          <div className="mb-2 flex items-center justify-between">
            <p className="text-xs font-bold text-[#004ac6]">
              {usage ? PLAN_LABELS[usage.plan] || usage.plan : "—"}
            </p>
            {usage && usage.plan !== "pro" && (
              <Link
                href="/plans"
                className="text-[10px] font-bold text-[#004ac6] hover:underline"
              >
                Upgrade
              </Link>
            )}
          </div>
          <div className="mb-2 h-1.5 w-full overflow-hidden rounded-full bg-slate-200">
            <div
              className={cn(
                "h-full rounded-full transition-all duration-500",
                isOverLimit
                  ? "bg-red-500"
                  : isNearLimit
                    ? "bg-amber-500"
                    : "bg-[#004ac6]"
              )}
              style={{ width: `${usedPercent}%` }}
            />
          </div>
          <p className="text-[10px] text-slate-500">
            {usage
              ? `${usage.month_leads} / ${usage.limit} лидов в этом месяце`
              : "Загрузка..."}
          </p>
        </div>

        {/* Links */}
        <div className="space-y-1">
          <button className="flex w-full items-center gap-3 rounded-lg px-4 py-2 text-sm font-medium text-[#434655] transition-colors hover:text-[#0d1c2e]">
            <HelpCircle className="size-5" />
            Поддержка
          </button>
          <button
            onClick={handleLogout}
            className="flex w-full items-center gap-3 rounded-lg px-4 py-2 text-sm font-medium text-[#434655] transition-colors hover:text-[#ba1a1a]"
          >
            <LogOut className="size-5" />
            Выход
          </button>
        </div>
      </div>
    </>
  );

  return (
    <>
      {/* Mobile hamburger button */}
      <button
        onClick={() => setMobileOpen(true)}
        className="fixed left-4 top-4 z-40 flex size-10 items-center justify-center rounded-lg bg-white shadow-md lg:hidden"
        aria-label="Open menu"
      >
        <Menu className="size-5 text-[#0d1c2e]" />
      </button>

      {/* Mobile overlay + sidebar */}
      {mobileOpen && (
        <div className="fixed inset-0 z-50 lg:hidden">
          {/* Backdrop */}
          <div
            className="absolute inset-0 bg-black/40"
            onClick={() => setMobileOpen(false)}
          />
          {/* Sliding sidebar */}
          <aside className="relative flex h-full w-64 flex-col bg-[#eff4ff] shadow-xl animate-in slide-in-from-left duration-200">
            {/* Close button */}
            <button
              onClick={() => setMobileOpen(false)}
              className="absolute right-3 top-3 flex size-8 items-center justify-center rounded-lg text-[#434655] hover:bg-[#dce9ff]"
              aria-label="Close menu"
            >
              <X className="size-5" />
            </button>
            {sidebarContent}
          </aside>
        </div>
      )}

      {/* Desktop sidebar — always visible on lg+ */}
      <aside className="hidden w-64 shrink-0 flex-col bg-[#eff4ff] lg:flex">
        {sidebarContent}
      </aside>
    </>
  );
}
