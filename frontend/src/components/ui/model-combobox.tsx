"use client";

import { useState, useEffect, useRef, useMemo } from "react";
import { ChevronDown } from "lucide-react";
import type { AIModelOption } from "@/lib/api";

interface ModelComboboxProps {
  value: string;
  onChange: (model: string) => void;
  options: AIModelOption[];
  loading?: boolean;
  placeholder?: string;
}

// ModelCombobox is a searchable model picker (#229): autocompletes from the
// provider's model list, filters as you type (typeahead), supports arrow-key
// navigation + Enter/Esc, and always allows a free-text custom model as a
// fallback (typed text that matches no option is committed on Enter).
export function ModelCombobox({ value, onChange, options, loading, placeholder }: ModelComboboxProps) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [highlight, setHighlight] = useState(0);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return options;
    return options.filter(
      (o) => o.id.toLowerCase().includes(q) || (o.meta ?? "").toLowerCase().includes(q)
    );
  }, [options, search]);

  // A free-text row appears when the search text is a non-empty custom value
  // that isn't already an exact option — the manual-entry fallback.
  const trimmed = search.trim();
  const showCustom = trimmed.length > 0 && !options.some((o) => o.id === trimmed);
  const rowCount = filtered.length + (showCustom ? 1 : 0);

  const commit = (model: string) => {
    if (!model) return;
    onChange(model);
    setOpen(false);
    setSearch("");
    setHighlight(0);
  };

  const selectRow = (idx: number) => {
    if (showCustom && idx === filtered.length) commit(trimmed);
    else if (filtered[idx]) commit(filtered[idx].id);
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setHighlight((h) => Math.min(h + 1, rowCount - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setHighlight((h) => Math.max(h - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      selectRow(highlight);
    } else if (e.key === "Escape") {
      setOpen(false);
    }
  };

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => { setOpen(!open); setSearch(""); setHighlight(0); }}
        className="flex w-full items-center justify-between rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm text-left outline-none transition-all focus:ring-2 focus:ring-[#3e3fcc]/20"
      >
        <span className={value ? "text-[#0d1c2e]" : "text-slate-400"}>
          {value || placeholder || "Выберите модель"}
        </span>
        <ChevronDown className="size-4 text-slate-400" />
      </button>

      {open && (
        <div className="absolute z-50 mt-1 w-full rounded-lg border border-[#c3c6d7]/20 bg-white shadow-lg">
          <div className="border-b border-[#c3c6d7]/10 p-2">
            <input
              type="text"
              placeholder="Поиск модели..."
              value={search}
              onChange={(e) => { setSearch(e.target.value); setHighlight(0); }}
              onKeyDown={onKeyDown}
              className="w-full rounded-md bg-[#eff4ff] px-3 py-1.5 text-sm outline-none placeholder-slate-400"
              autoFocus
            />
          </div>

          <div className="max-h-60 overflow-y-auto p-1">
            {loading && <p className="px-3 py-2 text-xs text-slate-400">Загрузка моделей…</p>}

            {!loading && filtered.map((o, i) => (
              <button
                key={o.id}
                type="button"
                role="option"
                aria-selected={i === highlight}
                onClick={() => commit(o.id)}
                onMouseEnter={() => setHighlight(i)}
                className={`flex w-full items-center justify-between rounded-md px-3 py-1.5 text-left text-sm transition-colors ${
                  i === highlight ? "bg-[#dbe1ff]" : "hover:bg-[#eff4ff]"
                } ${o.id === value ? "font-medium text-[#004ac6]" : "text-[#0d1c2e]"}`}
              >
                <span>{o.id}</span>
                {o.meta && <span className="ml-2 text-xs text-[#737686]">{o.meta}</span>}
              </button>
            ))}

            {!loading && showCustom && (
              <button
                type="button"
                role="option"
                aria-selected={highlight === filtered.length}
                onClick={() => commit(trimmed)}
                onMouseEnter={() => setHighlight(filtered.length)}
                className={`w-full rounded-md px-3 py-1.5 text-left text-sm transition-colors ${
                  highlight === filtered.length ? "bg-[#dbe1ff]" : "hover:bg-[#eff4ff]"
                } text-[#0d1c2e]`}
              >
                Свой вариант: «{trimmed}»
              </button>
            )}

            {!loading && rowCount === 0 && (
              <p className="px-3 py-2 text-xs text-slate-400">Начните вводить название модели</p>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
