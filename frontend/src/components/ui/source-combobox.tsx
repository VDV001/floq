"use client";

import { useState, useEffect, useRef } from "react";
import { ChevronDown, Plus, X } from "lucide-react";
import { api, type SourceCategory } from "@/lib/api";

interface SourceComboboxProps {
  value: string | null;
  onChange: (sourceId: string | null) => void;
}

export function SourceCombobox({ value, onChange }: SourceComboboxProps) {
  const [categories, setCategories] = useState<SourceCategory[]>([]);
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState("");
  const [newCategoryId, setNewCategoryId] = useState("");
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    api.getSources().then(setCategories).catch(() => {});
  }, []);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
        setCreating(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  const allSources = categories.flatMap((c) =>
    c.sources.map((s) => ({ ...s, categoryName: c.name }))
  );

  const selectedSource = allSources.find((s) => s.id === value);

  const filtered = search
    ? allSources.filter((s) =>
        s.name.toLowerCase().includes(search.toLowerCase()) ||
        s.categoryName.toLowerCase().includes(search.toLowerCase())
      )
    : allSources;

  const groupedFiltered = categories
    .map((c) => ({
      ...c,
      sources: filtered.filter((s) => s.category_id === c.id),
    }))
    .filter((c) => c.sources.length > 0);

  const handleCreateSource = async () => {
    if (!newName || !newCategoryId) return;
    try {
      const src = await api.createSource(newCategoryId, newName);
      setCategories((prev) =>
        prev.map((c) =>
          c.id === newCategoryId
            ? { ...c, sources: [...c.sources, src] }
            : c
        )
      );
      onChange(src.id);
      setCreating(false);
      setNewName("");
      setOpen(false);
    } catch {
      /* ignore */
    }
  };

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex w-full items-center justify-between rounded-lg border-none bg-[#eff4ff] px-4 py-2.5 text-sm text-left outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
      >
        <span className={selectedSource ? "text-[#0d1c2e]" : "text-slate-400"}>
          {selectedSource ? selectedSource.name : "Выберите источник"}
        </span>
        <div className="flex items-center gap-1">
          {value && (
            <span
              onClick={(e) => {
                e.stopPropagation();
                onChange(null);
              }}
              className="rounded p-0.5 hover:bg-slate-200"
            >
              <X className="size-3.5 text-slate-400" />
            </span>
          )}
          <ChevronDown className="size-4 text-slate-400" />
        </div>
      </button>

      {open && (
        <div className="absolute z-50 mt-1 w-full rounded-lg border border-[#c3c6d7]/20 bg-white shadow-lg">
          <div className="border-b border-[#c3c6d7]/10 p-2">
            <input
              type="text"
              placeholder="Поиск..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full rounded-md bg-[#eff4ff] px-3 py-1.5 text-sm outline-none placeholder-slate-400"
              autoFocus
            />
          </div>

          <div className="max-h-60 overflow-y-auto p-1">
            {groupedFiltered.map((cat) => (
              <div key={cat.id}>
                <div className="px-3 py-1.5 text-[10px] font-bold uppercase tracking-wider text-[#737686]">
                  {cat.name}
                </div>
                {cat.sources.map((src) => (
                  <button
                    key={src.id}
                    type="button"
                    onClick={() => {
                      onChange(src.id);
                      setOpen(false);
                      setSearch("");
                    }}
                    className={`w-full rounded-md px-3 py-1.5 text-left text-sm transition-colors hover:bg-[#eff4ff] ${
                      src.id === value ? "bg-[#dbe1ff] font-medium text-[#004ac6]" : "text-[#0d1c2e]"
                    }`}
                  >
                    {src.name}
                  </button>
                ))}
              </div>
            ))}

            {groupedFiltered.length === 0 && !creating && (
              <p className="px-3 py-2 text-xs text-slate-400">Ничего не найдено</p>
            )}
          </div>

          <div className="border-t border-[#c3c6d7]/10 p-2">
            {!creating ? (
              <button
                type="button"
                onClick={() => {
                  setCreating(true);
                  if (categories.length > 0 && !newCategoryId) {
                    setNewCategoryId(categories[0].id);
                  }
                }}
                className="flex w-full items-center gap-2 rounded-md px-3 py-1.5 text-sm text-[#004ac6] hover:bg-[#eff4ff]"
              >
                <Plus className="size-4" />
                Добавить источник
              </button>
            ) : (
              <div className="space-y-2">
                <select
                  value={newCategoryId}
                  onChange={(e) => setNewCategoryId(e.target.value)}
                  className="w-full rounded-md bg-[#eff4ff] px-3 py-1.5 text-sm outline-none"
                >
                  {categories.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.name}
                    </option>
                  ))}
                </select>
                <input
                  type="text"
                  placeholder="Название источника"
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  className="w-full rounded-md bg-[#eff4ff] px-3 py-1.5 text-sm outline-none placeholder-slate-400"
                  autoFocus
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      handleCreateSource();
                    }
                  }}
                />
                <div className="flex gap-2">
                  <button
                    type="button"
                    onClick={handleCreateSource}
                    className="flex-1 rounded-md bg-[#004ac6] px-3 py-1.5 text-xs font-semibold text-white"
                  >
                    Создать
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      setCreating(false);
                      setNewName("");
                    }}
                    className="rounded-md px-3 py-1.5 text-xs text-slate-500 hover:bg-slate-100"
                  >
                    Отмена
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
