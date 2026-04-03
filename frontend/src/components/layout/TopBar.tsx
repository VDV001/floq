"use client";

import { usePathname } from "next/navigation";
import Link from "next/link";
import { Search, Bell, Settings, Plus } from "lucide-react";
import { cn } from "@/lib/utils";

const TABS = [
  { label: "Входящие", href: "/inbox" },
  { label: "Воронка", href: "/pipeline" },
  { label: "Аналитика", href: "/analytics" },
];

export function TopBar() {
  const pathname = usePathname();

  return (
    <header className="flex items-center justify-between border-b border-[#e5e7eb] bg-white px-6 py-3">
      {/* Left: Search */}
      <div className="flex items-center gap-4">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[#6b7280]" />
          <input
            type="text"
            placeholder="Поиск лидов, компаний, сообщений..."
            className="h-9 w-80 rounded-lg border border-[#e5e7eb] bg-[#f8f9ff] pl-9 pr-3 text-sm text-[#0d1c2e] placeholder:text-[#6b7280] outline-none transition-colors focus:border-[#3b6ef6] focus:ring-1 focus:ring-[#3b6ef6]/30"
          />
        </div>

        {/* Tabs */}
        <nav className="flex items-center gap-1">
          {TABS.map((tab) => {
            const isActive = pathname.startsWith(tab.href);
            return (
              <Link
                key={tab.href}
                href={tab.href}
                className={cn(
                  "relative px-3 py-2 text-sm font-medium transition-colors",
                  isActive
                    ? "text-[#3b6ef6]"
                    : "text-[#6b7280] hover:text-[#0d1c2e]"
                )}
              >
                {tab.label}
                {isActive && (
                  <span className="absolute bottom-0 left-0 right-0 h-0.5 rounded-full bg-[#3b6ef6]" />
                )}
              </Link>
            );
          })}
        </nav>
      </div>

      {/* Right: Actions */}
      <div className="flex items-center gap-3">
        <button className="flex size-9 items-center justify-center rounded-lg text-[#6b7280] transition-colors hover:bg-gray-100">
          <Bell className="size-4" />
        </button>
        <button className="flex size-9 items-center justify-center rounded-lg text-[#6b7280] transition-colors hover:bg-gray-100">
          <Settings className="size-4" />
        </button>
        <button className="flex items-center gap-1.5 rounded-lg bg-[#3b6ef6] px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-[#3b6ef6]/90">
          <Plus className="size-4" />
          Новый лид
        </button>
        <div className="flex size-8 items-center justify-center rounded-full bg-[#3b6ef6] text-xs font-medium text-white">
          DK
        </div>
      </div>
    </header>
  );
}
