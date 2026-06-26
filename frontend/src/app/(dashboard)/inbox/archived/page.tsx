"use client";

import { useState } from "react";
import Link from "next/link";
import { ArrowLeft, Archive } from "lucide-react";
import { ArchivedLeadCard } from "@/components/leads/ArchivedLeadCard";
import { mapStatus } from "@/components/inbox/constants";
import { getTimeAgo } from "@/lib/format";
import { useArchivedLeads } from "@/hooks/useArchivedLeads";
import { useNotify } from "@/components/notifications/NotificationProvider";
import { unarchiveLead } from "@/lib/leadActions";

// archivedLabel phrases the "when archived" badge. getTimeAgo already reads
// "Только что" for sub-minute gaps, which must NOT get a trailing "назад"
// ("Только что назад" is broken Russian); every other bucket does.
function archivedLabel(dateStr: string): string {
  const ago = getTimeAgo(dateStr);
  return ago === "Только что" ? ago : `${ago} назад`;
}

export default function ArchivedLeadsPage() {
  const { loading, leads, error, removeLead } = useArchivedLeads();
  const { notify, notifyError } = useNotify();
  // Tracks which leads are mid-unarchive so each row shows its own spinner and
  // blocks its own double-clicks — without freezing unarchive on the others.
  const [pendingIds, setPendingIds] = useState<Set<string>>(new Set());

  async function handleUnarchive(id: string) {
    if (pendingIds.has(id)) return;
    setPendingIds((prev) => new Set(prev).add(id));
    const ok = await unarchiveLead(id, notify, notifyError);
    if (ok) removeLead(id);
    setPendingIds((prev) => {
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
  }

  return (
    <section className="flex-1 overflow-y-auto px-4 sm:px-8 lg:px-12 py-8">
      <div className="mx-auto max-w-4xl space-y-8">
        <Link
          href="/inbox"
          className="inline-flex items-center gap-1.5 text-sm text-[#434655] transition-colors hover:text-[#004ac6]"
        >
          <ArrowLeft className="size-4" /> К ленте лидов
        </Link>

        <div>
          <div className="flex items-center gap-3">
            <Archive className="size-6 text-[#434655]" />
            <h2 className="text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">Архив лидов</h2>
            {loading && <div className="size-5 animate-spin rounded-full border-2 border-[#3b6ef6] border-t-transparent" />}
          </div>
          <p className="mt-1 text-sm text-[#434655]">
            Скрытые из ленты и аналитики лиды. Разархивируйте, чтобы вернуть лид в работу.
          </p>
        </div>

        <div className="space-y-3">
          {!loading && error && (
            <div className="rounded-xl bg-white p-12 text-center">
              <p className="text-lg font-bold text-[#0d1c2e]">Не удалось загрузить архив</p>
              <p className="mt-2 text-sm text-[#434655]">
                Проверьте соединение и обновите страницу. Архивные лиды никуда не делись.
              </p>
            </div>
          )}
          {!loading && !error && leads.length === 0 && (
            <div className="rounded-xl bg-white p-12 text-center">
              <p className="text-lg font-bold text-[#0d1c2e]">Архив пуст</p>
              <p className="mt-2 text-sm text-[#434655]">
                Заархивированные лиды появятся здесь. Архивировать лид можно на его странице.
              </p>
            </div>
          )}
          {!error && leads.map((lead) => (
            <ArchivedLeadCard
              key={lead.id}
              id={lead.id}
              company={lead.company || "—"}
              contact={lead.contact_name}
              channel={lead.channel}
              status={mapStatus(lead.status)}
              sourceName={lead.source_name}
              archivedLabel={archivedLabel(lead.archived_at ?? lead.updated_at)}
              unarchiving={pendingIds.has(lead.id)}
              onUnarchive={handleUnarchive}
            />
          ))}
        </div>
      </div>
    </section>
  );
}
