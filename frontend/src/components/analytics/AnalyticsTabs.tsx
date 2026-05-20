"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";

const TABS = [
  { href: "/analytics/sequences", label: "Sequences" },
  { href: "/analytics/cost", label: "Затраты" },
];

// AnalyticsTabs renders the sub-navigation for /analytics/* pages.
// Active state derived from usePathname() so adding a new view only
// needs an entry in TABS — no per-page wiring. Mirrors the
// PendingQueueTabs pattern used in the inbox surface.
export function AnalyticsTabs() {
  const pathname = usePathname();
  return (
    <nav aria-label="Аналитика" className="flex gap-2 border-b border-slate-200">
      {TABS.map((tab) => {
        const active = pathname === tab.href;
        return (
          <Link
            key={tab.href}
            href={tab.href}
            className={cn(
              "px-3 py-2 text-sm font-medium border-b-2 transition-colors -mb-px",
              active
                ? "border-slate-900 text-slate-900"
                : "border-transparent text-slate-500 hover:text-slate-700",
            )}
          >
            {tab.label}
          </Link>
        );
      })}
    </nav>
  );
}
