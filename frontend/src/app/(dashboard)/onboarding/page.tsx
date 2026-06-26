"use client";

import { cn } from "@/lib/utils";
import { useOnboarding } from "@/hooks/useOnboarding";
import { STEPS } from "@/components/onboarding/constants";
import { HowItWorks } from "@/components/onboarding/HowItWorks";
import { StepTimeline } from "@/components/onboarding/StepTimeline";
import { SectionsMap } from "@/components/onboarding/SectionsMap";
import { Glossary } from "@/components/onboarding/Glossary";
import { TipsSection } from "@/components/onboarding/TipsSection";
import { FooterBanner } from "@/components/onboarding/FooterBanner";

export default function OnboardingPage() {
  const { settings, counts, loading, completedSteps, progress, allDone } =
    useOnboarding();

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
        {/* Header */}
        <header className="mb-14">
          <div className="mb-1 text-sm font-bold uppercase tracking-widest text-[#004ac6]/60">
            Начало работы
          </div>
          <h2 className="mb-3 text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">
            {allDone ? "Всё готово!" : "Добро пожаловать в Floq"}
          </h2>
          <p className="mb-8 text-lg text-[#434655]">
            {allDone
              ? "Настройка завершена. Ваш AI-ассистент готов к работе. Ниже — как всё устроено, карта разделов и словарь терминов."
              : "Полное руководство: разберитесь, как устроена система, пройдите настройку по шагам и запустите первую рассылку. Начните сверху и двигайтесь вниз."}
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

        <HowItWorks />

        <div className="mb-8">
          <h3 className="mb-2 text-xs font-bold uppercase tracking-widest text-[#434655]/50">
            Пошаговая настройка
          </h3>
          <p className="text-sm leading-relaxed text-[#434655]/80">
            Семь шагов от пустого аккаунта до первой отправленной рассылки.
            Раскройте «Подробную инструкцию» под каждым шагом — там всё разжёвано.
          </p>
        </div>
        <StepTimeline settings={settings} counts={counts} />

        <SectionsMap />
        <Glossary />
        <TipsSection />
        <FooterBanner />
      </div>
    </div>
  );
}
