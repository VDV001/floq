"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import {
  Send,
  Sparkles,
  UserPlus,
  GitBranch,
  Rocket,
  Check,
  ArrowRight,
  Brain,
  Shield,
  Bell,
  MessageCircle,
  Mail,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { api, type UserSettings } from "@/lib/api";

interface StepDef {
  id: string;
  icon: React.ElementType;
  title: string;
  description: string;
  href: string;
  btnLabel: string;
  check: (s: UserSettings, counts: Counts) => boolean;
}

interface Counts {
  prospects: number;
  sequences: number;
  outbound: number;
}

const STEPS: StepDef[] = [
  {
    id: "telegram",
    icon: Send,
    title: "Подключите Telegram бота",
    description:
      "Создайте бота через @BotFather, скопируйте токен и вставьте в настройках. Входящие сообщения автоматически станут лидами.",
    href: "/settings",
    btnLabel: "Настроить",
    check: (s) => s.telegram_bot_active,
  },
  {
    id: "ai",
    icon: Sparkles,
    title: "Настройте AI-провайдер",
    description:
      "Выберите Claude, OpenAI, Groq или Ollama. AI будет квалифицировать лидов, генерировать ответы и холодные сообщения.",
    href: "/settings",
    btnLabel: "Настроить",
    check: (s) => s.ai_active,
  },
  {
    id: "email-out",
    icon: Mail,
    title: "Настройте отправку писем",
    description:
      "Подключите SMTP (mail.ru, Яндекс, Gmail) или Resend API для отправки холодных писем из секвенций.",
    href: "/settings",
    btnLabel: "Настроить",
    check: (s) => s.smtp_active || s.resend_active,
  },
  {
    id: "email-in",
    icon: Mail,
    title: "Подключите приём почты (IMAP)",
    description:
      "Настройте IMAP для автоматического приёма входящих писем. Ответы от проспектов будут создавать лидов автоматически.",
    href: "/settings",
    btnLabel: "Настроить",
    check: (s) => s.imap_active,
  },
  {
    id: "prospects",
    icon: UserPlus,
    title: "Добавьте первых проспектов",
    description:
      "Импортируйте CSV-файл с контактами или добавьте вручную. Верификация email встроена — плохие адреса отсеются.",
    href: "/prospects",
    btnLabel: "Добавить",
    check: (_, c) => c.prospects > 0,
  },
  {
    id: "sequence",
    icon: GitBranch,
    title: "Создайте секвенцию",
    description:
      "Настройте цепочку касаний: Email на старте → Telegram через 3 дня → Прозвон через неделю. AI напишет текст под каждый канал.",
    href: "/sequences",
    btnLabel: "Создать",
    check: (_, c) => c.sequences > 0,
  },
  {
    id: "launch",
    icon: Rocket,
    title: "Запустите первую рассылку",
    description:
      "Выберите проспектов, запустите секвенцию. Сообщения попадут в очередь одобрения — вы контролируете каждое касание.",
    href: "/outbound",
    btnLabel: "Запустить",
    check: (_, c) => c.outbound > 0,
  },
];

const TIPS = [
  {
    icon: Brain,
    title: "AI-квалификация",
    description:
      "Каждый входящий лид оценивается по потребности, бюджету и срокам. Скор от 0 до 100 помогает фокусироваться на горячих.",
    accent: "from-[#004ac6] to-[#2563eb]",
  },
  {
    icon: Shield,
    title: "Верификация контактов",
    description:
      "Встроенный SMTP probe, MX lookup и фильтр одноразовых доменов. Проверяйте email до отправки — без платных сервисов.",
    accent: "from-[#059669] to-[#10b981]",
  },
  {
    icon: Bell,
    title: "Автоматические фоллоуапы",
    description:
      "Если лид молчит 2+ дня — Floq напомнит в Telegram. Ни один контакт не потеряется в воронке.",
    accent: "from-[#d97706] to-[#f59e0b]",
  },
];

export default function OnboardingPage() {
  const [settings, setSettings] = useState<UserSettings | null>(null);
  const [counts, setCounts] = useState<Counts>({
    prospects: 0,
    sequences: 0,
    outbound: 0,
  });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.all([
      api.getSettings(),
      api.getProspects().then((p) => p.length).catch(() => 0),
      api.getSequences().then((s) => s.length).catch(() => 0),
      api.getOutboundQueue().then((q) => q.length).catch(() => 0),
    ])
      .then(([s, prospects, sequences, outbound]) => {
        setSettings(s);
        setCounts({ prospects, sequences, outbound });
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const completedSteps = settings
    ? STEPS.filter((step) => step.check(settings, counts)).length
    : 0;
  const progress = (completedSteps / STEPS.length) * 100;
  const allDone = completedSteps === STEPS.length;

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="size-8 animate-spin rounded-full border-2 border-[#004ac6] border-t-transparent" />
      </div>
    );
  }

  return (
    <div className="min-h-full pb-16 pt-20">
      <div className="mx-auto max-w-3xl px-8">
        {/* ── Header ── */}
        <header className="mb-14">
          <div className="mb-1 text-sm font-bold uppercase tracking-widest text-[#004ac6]/60">
            Начало работы
          </div>
          <h2 className="mb-3 text-2xl sm:text-3xl lg:text-[2.5rem] font-extrabold leading-tight tracking-tight text-[#0d1c2e]">
            {allDone ? "Всё готово!" : "Добро пожаловать в Floq"}
          </h2>
          <p className="mb-8 text-lg text-[#434655]">
            {allDone
              ? "Настройка завершена. Ваш AI-ассистент готов к работе."
              : "Настройте систему за 5 минут и начните получать лиды."}
          </p>

          {/* Progress */}
          <div className="flex items-center gap-4">
            <div className="h-2 flex-1 overflow-hidden rounded-full bg-[#dbe1ff]">
              <div
                className={cn(
                  "h-full rounded-full transition-all duration-700 ease-out",
                  allDone
                    ? "bg-gradient-to-r from-green-400 to-emerald-500"
                    : "bg-gradient-to-r from-[#004ac6] to-[#2563eb]"
                )}
                style={{ width: `${progress}%` }}
              />
            </div>
            <span className="shrink-0 text-sm font-bold text-[#0d1c2e]">
              {completedSteps} / {STEPS.length}
            </span>
          </div>
        </header>

        {/* ── Steps timeline ── */}
        <section className="relative mb-20">
          {/* Vertical line */}
          <div className="absolute left-[23px] top-6 bottom-6 w-px bg-[#dbe1ff]" />

          <div className="space-y-2">
            {STEPS.map((step, i) => {
              const done = settings
                ? step.check(settings, counts)
                : false;
              const isNext =
                !done &&
                (i === 0 ||
                  (settings
                    ? STEPS[i - 1].check(settings, counts)
                    : false));

              return (
                <div
                  key={step.id}
                  className={cn(
                    "group relative flex gap-5 rounded-2xl p-5 transition-all duration-300",
                    done
                      ? "bg-white/40"
                      : isNext
                        ? "bg-white shadow-sm shadow-[#004ac6]/5 ring-1 ring-[#004ac6]/10"
                        : "bg-transparent opacity-60"
                  )}
                >
                  {/* Step indicator */}
                  <div
                    className={cn(
                      "relative z-10 flex size-[46px] shrink-0 items-center justify-center rounded-xl transition-all duration-300",
                      done
                        ? "bg-gradient-to-br from-green-400 to-emerald-500 text-white shadow-md shadow-green-500/25"
                        : isNext
                          ? "bg-gradient-to-br from-[#004ac6] to-[#2563eb] text-white shadow-md shadow-[#004ac6]/25"
                          : "bg-[#eff4ff] text-[#434655] ring-1 ring-[#c3c6d7]/20"
                    )}
                  >
                    {done ? (
                      <Check className="size-5" strokeWidth={3} />
                    ) : (
                      <step.icon className="size-5" />
                    )}
                  </div>

                  {/* Content */}
                  <div className="flex-1 pt-0.5">
                    <div className="mb-1 flex items-center gap-3">
                      <h3
                        className={cn(
                          "text-base font-bold",
                          done
                            ? "text-[#434655] line-through decoration-green-400 decoration-2"
                            : "text-[#0d1c2e]"
                        )}
                      >
                        {step.title}
                      </h3>
                      {done && (
                        <span className="rounded-full bg-green-100 px-2.5 py-0.5 text-[10px] font-bold uppercase tracking-wider text-green-700">
                          Готово
                        </span>
                      )}
                    </div>
                    <p className="mb-3 text-sm leading-relaxed text-[#434655]/80">
                      {step.description}
                    </p>
                    {!done && (
                      <Link
                        href={step.href}
                        className={cn(
                          "inline-flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-bold transition-all",
                          isNext
                            ? "bg-gradient-to-r from-[#004ac6] to-[#2563eb] text-white shadow-md shadow-[#004ac6]/20 hover:-translate-y-0.5 hover:shadow-lg"
                            : "bg-[#eff4ff] text-[#004ac6] hover:bg-[#dbe1ff]"
                        )}
                      >
                        {step.btnLabel}
                        <ArrowRight className="size-3.5" />
                      </Link>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </section>

        {/* ── Tips ── */}
        <section className="mb-16">
          <h3 className="mb-6 text-xs font-bold uppercase tracking-widest text-[#434655]/50">
            Полезные возможности
          </h3>
          <div className="grid gap-4 sm:grid-cols-3">
            {TIPS.map((tip) => (
              <div
                key={tip.title}
                className="group rounded-2xl bg-white p-6 shadow-sm ring-1 ring-[#c3c6d7]/10 transition-all duration-300 hover:-translate-y-0.5 hover:shadow-md"
              >
                <div
                  className={cn(
                    "mb-4 flex size-10 items-center justify-center rounded-lg bg-gradient-to-br text-white",
                    tip.accent
                  )}
                >
                  <tip.icon className="size-5" />
                </div>
                <h4 className="mb-2 text-sm font-bold text-[#0d1c2e]">
                  {tip.title}
                </h4>
                <p className="text-xs leading-relaxed text-[#434655]/80">
                  {tip.description}
                </p>
              </div>
            ))}
          </div>
        </section>

        {/* ── Footer banner ── */}
        <section className="overflow-hidden rounded-2xl bg-gradient-to-br from-[#004ac6] to-[#2563eb] p-8 shadow-xl shadow-[#004ac6]/15">
          <div className="flex flex-col items-center gap-6 text-center sm:flex-row sm:text-left">
            <div className="flex-1">
              <h3 className="mb-2 text-lg font-extrabold text-white">
                Нужна помощь с настройкой?
              </h3>
              <p className="text-sm text-white/70">
                Мы поможем подключить все каналы и запустить первую рассылку.
                Бесплатно для всех тарифов.
              </p>
            </div>
            <div className="flex shrink-0 gap-3">
              <a
                href="https://t.me/floq_support"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 rounded-xl bg-white/15 px-5 py-3 text-sm font-bold text-white backdrop-blur-sm transition-all hover:bg-white/25"
              >
                <MessageCircle className="size-4" />
                Telegram
              </a>
              <a
                href="mailto:support@floq.ai"
                className="flex items-center gap-2 rounded-xl bg-white px-5 py-3 text-sm font-bold text-[#004ac6] shadow-md transition-all hover:-translate-y-0.5 hover:shadow-lg"
              >
                <Mail className="size-4" />
                Email
              </a>
            </div>
          </div>
        </section>
      </div>
    </div>
  );
}
