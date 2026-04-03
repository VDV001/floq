"use client";

import { useState } from "react";
import { Sparkles, X, Send } from "lucide-react";

export function FloatingActionButton() {
  const [open, setOpen] = useState(false);

  return (
    <>
      {/* Chat panel */}
      {open && (
        <div className="fixed bottom-24 right-6 z-50 flex w-96 flex-col overflow-hidden rounded-2xl border border-[#c3c6d7]/20 bg-white shadow-2xl">
          {/* Header */}
          <div className="flex items-center justify-between px-5 py-4">
            <div className="flex items-center gap-3">
              <Sparkles className="size-6 text-[#3b6ef6]" />
              <div>
                <p className="text-sm font-bold text-[#0d1c2e]">Floq AI</p>
                <p className="text-[11px] text-[#434655]">Ассистент по продажам</p>
              </div>
            </div>
            <button
              onClick={() => setOpen(false)}
              className="rounded-lg p-1 text-[#434655] transition-colors hover:bg-[#eff4ff] hover:text-[#0d1c2e]"
            >
              <X className="size-5" />
            </button>
          </div>

          {/* Body */}
          <div className="flex flex-1 flex-col items-center justify-center px-6 py-12 text-center">
            <div className="mb-4 flex size-16 items-center justify-center rounded-full bg-[#eff4ff]">
              <Sparkles className="size-8 text-[#3b6ef6]" />
            </div>
            <h3 className="mb-2 text-lg font-bold text-[#0d1c2e]">
              AI-ассистент
            </h3>
            <p className="text-sm leading-relaxed text-[#434655]">
              Скоро здесь будет чат с AI-ассистентом. Задавайте вопросы по лидам,
              воронке и аналитике — AI ответит на основе ваших данных.
            </p>
            <span className="mt-4 rounded-full bg-[#e1e0ff] px-3 py-1 text-xs font-bold text-[#3e3fcc]">
              Coming soon
            </span>
          </div>

          {/* Input (disabled) */}
          <div className="p-4">
            <div className="flex items-center gap-2">
              <input
                type="text"
                placeholder="Спросите что-нибудь..."
                disabled
                className="flex-1 rounded-xl bg-[#eff4ff] px-4 py-2.5 text-sm placeholder-[#737686] outline-none disabled:opacity-50"
              />
              <button
                disabled
                className="flex size-10 items-center justify-center rounded-xl bg-[#3b6ef6] text-white opacity-50"
              >
                <Send className="size-4" />
              </button>
            </div>
          </div>
        </div>
      )}

      {/* FAB button */}
      <button
        onClick={() => setOpen(!open)}
        className="fixed bottom-6 right-6 z-50 flex size-14 items-center justify-center rounded-full bg-[#3b6ef6] text-white shadow-lg transition-all hover:scale-105 hover:shadow-xl"
      >
        {open ? <X className="size-6" /> : <Sparkles className="size-6" />}
      </button>
    </>
  );
}
