"use client";

import { useState, useEffect } from "react";
import { Check, Sparkles, Zap, Crown } from "lucide-react";
import { cn } from "@/lib/utils";
import { api } from "@/lib/api";

const PLANS = [
  {
    id: "starter",
    name: "Starter",
    price: "3 900",
    period: "/ мес",
    description: "Для начинающих команд продаж",
    icon: Zap,
    color: "from-slate-500 to-slate-700",
    ring: "ring-slate-200",
    badge: null,
    features: [
      "До 200 лидов в месяц",
      "Единый inbox (TG + Email)",
      "AI-квалификация лидов",
      "AI-черновики ответов",
      "Напоминания о фоллоуапах",
      "1 пользователь",
    ],
    limits: { leads: 200 },
  },
  {
    id: "growth",
    name: "Growth",
    price: "7 900",
    period: "/ мес",
    description: "Для растущих отделов продаж",
    icon: Sparkles,
    color: "from-[#004ac6] to-[#2563eb]",
    ring: "ring-[#004ac6]/30",
    badge: "Популярный",
    features: [
      "До 1 000 лидов в месяц",
      "Всё из Starter",
      "Мультиканальные секвенции",
      "Холодный аутрич с AI",
      "Email-верификатор",
      "Парсинг 2GIS + сайтов",
      "Очередь одобрения",
      "Трекинг открытий и ответов",
    ],
    limits: { leads: 1000 },
  },
  {
    id: "pro",
    name: "Pro",
    price: "14 900",
    period: "/ мес",
    description: "Для профессиональных команд",
    icon: Crown,
    color: "from-[#7c3aed] to-[#a855f7]",
    ring: "ring-[#7c3aed]/30",
    badge: "Максимум",
    features: [
      "Безлимитные лиды",
      "Всё из Growth",
      "API-доступ",
      "Приоритетная поддержка",
      "Кастомные AI-промпты",
      "Webhook-интеграции",
      "Выделенный менеджер",
    ],
    limits: { leads: Infinity },
  },
];

export default function PlansPage() {
  const [currentPlan, setCurrentPlan] = useState<string>("growth");
  const [usage, setUsage] = useState<{ month_leads: number; limit: number } | null>(null);

  useEffect(() => {
    api.getUsage().then((data) => {
      setCurrentPlan(data.plan);
      setUsage({ month_leads: data.month_leads, limit: data.limit });
    }).catch(() => {});
  }, []);

  return (
    <div className="min-h-full px-4 sm:px-8 lg:px-12 pb-16 pt-8 sm:pt-16 lg:pt-24">
      <div className="mx-auto max-w-5xl">
        {/* Header */}
        <header className="mb-12 text-center">
          <h2 className="text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">
            Тарифные планы
          </h2>
          <p className="mt-3 text-lg text-[#434655]">
            Выберите план, который подходит вашей команде
          </p>
          {usage && (
            <p className="mt-2 text-sm text-[#434655]/70">
              Сейчас:{" "}
              <span className="font-semibold text-[#0d1c2e]">
                {PLANS.find((p) => p.id === currentPlan)?.name || currentPlan}
              </span>
              {" — "}
              {usage.month_leads} / {usage.limit} лидов использовано в этом месяце
            </p>
          )}
        </header>

        {/* Plan cards */}
        <div className="grid gap-8 lg:grid-cols-3">
          {PLANS.map((plan) => {
            const isCurrent = plan.id === currentPlan;
            const isPopular = plan.badge === "Популярный";

            return (
              <div
                key={plan.id}
                className={cn(
                  "relative flex flex-col rounded-2xl bg-white p-8 shadow-sm transition-all duration-300 hover:-translate-y-1 hover:shadow-xl",
                  isPopular
                    ? "ring-2 ring-[#004ac6]/30 shadow-lg shadow-[#004ac6]/10"
                    : "ring-1 ring-[#c3c6d7]/15"
                )}
              >
                {/* Badge */}
                {plan.badge && (
                  <div
                    className={cn(
                      "absolute -top-3.5 left-1/2 -translate-x-1/2 rounded-full px-4 py-1 text-[10px] font-bold uppercase tracking-wider text-white shadow-md",
                      `bg-gradient-to-r ${plan.color}`
                    )}
                  >
                    {plan.badge}
                  </div>
                )}

                {/* Icon + Name */}
                <div className="mb-6 flex items-center gap-3">
                  <div
                    className={cn(
                      "flex size-12 items-center justify-center rounded-xl bg-gradient-to-br text-white shadow-md",
                      plan.color
                    )}
                  >
                    <plan.icon className="size-6" />
                  </div>
                  <div>
                    <h3 className="text-xl font-extrabold text-[#0d1c2e]">
                      {plan.name}
                    </h3>
                    <p className="text-xs text-[#434655]">{plan.description}</p>
                  </div>
                </div>

                {/* Price */}
                <div className="mb-8">
                  <div className="flex items-baseline gap-1">
                    <span className="text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">
                      {plan.price}
                    </span>
                    <span className="text-lg font-semibold text-[#434655]">
                      ₽
                    </span>
                    <span className="ml-1 text-sm text-[#434655]/70">
                      {plan.period}
                    </span>
                  </div>
                </div>

                {/* Features */}
                <ul className="mb-8 flex-1 space-y-3">
                  {plan.features.map((feature) => (
                    <li key={feature} className="flex items-start gap-3">
                      <Check
                        className={cn(
                          "mt-0.5 size-4 shrink-0",
                          isPopular ? "text-[#004ac6]" : "text-green-500"
                        )}
                      />
                      <span className="text-sm text-[#434655]">{feature}</span>
                    </li>
                  ))}
                </ul>

                {/* CTA */}
                {isCurrent ? (
                  <button
                    disabled
                    className="flex w-full items-center justify-center gap-2 rounded-xl bg-[#eff4ff] py-3.5 text-sm font-bold text-[#004ac6]"
                  >
                    <Check className="size-4" />
                    Текущий план
                  </button>
                ) : (
                  <button
                    className={cn(
                      "w-full rounded-xl py-3.5 text-sm font-bold text-white shadow-lg transition-all hover:-translate-y-0.5 hover:shadow-xl active:scale-[0.98]",
                      `bg-gradient-to-r ${plan.color}`,
                      isPopular && "shadow-[#004ac6]/30"
                    )}
                  >
                    {plan.limits.leads > (usage?.limit ?? 0)
                      ? "Перейти на " + plan.name
                      : "Выбрать " + plan.name}
                  </button>
                )}
              </div>
            );
          })}
        </div>

        {/* FAQ / bottom note */}
        <div className="mt-16 rounded-2xl bg-[#eff4ff] p-8 text-center">
          <h3 className="mb-3 text-lg font-bold text-[#0d1c2e]">
            Есть вопросы?
          </h3>
          <p className="mb-4 text-sm text-[#434655]">
            Все планы включают бесплатную настройку и онбординг.
            Переключайтесь между тарифами в любой момент.
          </p>
          <p className="text-sm text-[#434655]">
            Напишите нам:{" "}
            <a
              href="mailto:support@floq.ai"
              className="font-semibold text-[#004ac6] hover:underline"
            >
              support@floq.ai
            </a>
          </p>
        </div>
      </div>
    </div>
  );
}
