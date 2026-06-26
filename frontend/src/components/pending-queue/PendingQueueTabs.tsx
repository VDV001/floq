"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/utils";

interface Props {
  /** Optional pending count shown in the second tab. Undefined hides
   *  the badge — call sites that already know the count (queue page)
   *  pass it; callers that don't (leads list) omit it. */
  pendingCount?: number;
}

/**
 * PendingQueueTabs renders the inbox sub-navigation: all leads vs
 * pending-approval queue. Sits at the top of /inbox and /inbox/pending
 * so operators can flip between the two without leaving the section.
 *
 * Active tab is derived from usePathname; nested routes under /inbox
 * (e.g. /inbox/[leadId]) keep the "All leads" tab highlighted because
 * /inbox/pending is the only sibling that flips it.
 */
export function PendingQueueTabs({ pendingCount }: Props) {
  const pathname = usePathname();
  const onPending = pathname === "/inbox/pending";

  return (
    <div className="mb-6 flex gap-1 rounded-xl bg-[#eff4ff] p-1">
      <Link
        href="/inbox"
        className={cn(
          "flex-1 rounded-lg px-4 py-2 text-center text-sm font-bold transition-all",
          !onPending
            ? "bg-white text-[#0d1c2e] shadow-sm"
            : "text-[#434655] hover:text-[#0d1c2e]"
        )}
      >
        Все лиды
      </Link>
      <Link
        href="/inbox/pending"
        className={cn(
          "flex-1 rounded-lg px-4 py-2 text-center text-sm font-bold transition-all",
          onPending
            ? "bg-white text-[#0d1c2e] shadow-sm"
            : "text-[#434655] hover:text-[#0d1c2e]"
        )}
      >
        Очередь HITL{pendingCount !== undefined ? ` (${pendingCount})` : ""}
      </Link>
    </div>
  );
}
