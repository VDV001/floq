"use client";

import { useState } from "react";
import Link from "next/link";
import { GraduationCap, ArrowRight, X } from "lucide-react";

// Shared localStorage flag: the banner hides once the user dismisses it OR
// finishes onboarding (the onboarding page sets the same key on allDone). This
// targets the nudge at new accounts without any extra API calls on the inbox —
// the hottest screen.
export const ONBOARDING_BANNER_HIDDEN_KEY = "floq_onboarding_banner_hidden";

function isHidden(): boolean {
  if (typeof window === "undefined") return false;
  try {
    return window.localStorage.getItem(ONBOARDING_BANNER_HIDDEN_KEY) === "1";
  } catch {
    return false;
  }
}

// OnboardingBanner is a dismissible nudge pointing new users to the /onboarding
// tutorial. It carries no onboarding state of its own (no network) — it simply
// hides once the user dismisses it or completes onboarding.
export function OnboardingBanner() {
  const [hidden, setHidden] = useState(isHidden);

  if (hidden) return null;

  const dismiss = () => {
    try {
      window.localStorage.setItem(ONBOARDING_BANNER_HIDDEN_KEY, "1");
    } catch {
      /* ignore storage failures — dismissing for this session is enough */
    }
    setHidden(true);
  };

  return (
    <div className="flex items-center gap-4 overflow-hidden rounded-2xl bg-gradient-to-br from-[#004ac6] to-[#2563eb] px-5 py-4 shadow-md shadow-[#004ac6]/15">
      <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-white/15 text-white backdrop-blur-sm">
        <GraduationCap className="size-5" />
      </div>
      <div className="min-w-0 flex-1">
        <p className="text-sm font-bold text-white">Впервые здесь? Пройдите обучение</p>
        <p className="text-xs text-white/70">
          Разберём, как устроена система, настроим каналы и запустим первую рассылку — пошагово.
        </p>
      </div>
      <Link
        href="/onboarding"
        className="flex shrink-0 items-center gap-1.5 rounded-lg bg-white px-4 py-2 text-xs font-bold text-[#004ac6] shadow-sm transition-all hover:-translate-y-0.5 hover:shadow-md"
      >
        Открыть обучение
        <ArrowRight className="size-3.5" />
      </Link>
      <button
        type="button"
        onClick={dismiss}
        aria-label="Скрыть подсказку об обучении"
        className="flex size-8 shrink-0 items-center justify-center rounded-lg text-white/60 transition-colors hover:bg-white/15 hover:text-white"
      >
        <X className="size-4" />
      </button>
    </div>
  );
}
