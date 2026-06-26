"use client";

import { useState } from "react";
import Link from "next/link";
import { ArrowLeft, Archive } from "lucide-react";
import { ArchivedLeadCard } from "@/components/leads/ArchivedLeadCard";
import { mapStatus } from "@/components/inbox/constants";
import { getTimeAgo } from "@/lib/format";
import { useArchivedLeads } from "@/hooks/useArchivedLeads";
import { useNotify } from "@/components/notifications/NotificationProvider";
import { api } from "@/lib/api";

export default function ArchivedLeadsPage() {
  const { loading, leads, removeLead } = useArchivedLeads();
  const { notify, notifyError } = useNotify();
  // Tracks which lead is mid-unarchive so its button can show a spinner and
  // block double-clicks without disabling the whole list.
  const [pendingId, setPendingId] = useState<string | null>(null);

  async function handleUnarchive(id: string) {
    if (pendingId) return;
    setPendingId(id);
    try {
      await api.unarchiveLead(id);
      removeLead(id);
      notify({ type: "success", title: "Лид возвращён", message: "Он снова в ленте входящих." });
    } catch (err) {
      notifyError(err, "Не удалось разархивировать лид");
    } finally {
      setPendingId(null);
    }
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
          {!loading && leads.length === 0 && (
            <div className="rounded-xl bg-white p-12 text-center">
              <p className="text-lg font-bold text-[#0d1c2e]">Архив пуст</p>
              <p className="mt-2 text-sm text-[#434655]">
                Заархивированные лиды появятся здесь. Архивировать лид можно на его странице.
              </p>
            </div>
          )}
          {leads.map((lead) => (
            <ArchivedLeadCard
              key={lead.id}
              id={lead.id}
              company={lead.company || "—"}
              contact={lead.contact_name}
              channel={lead.channel}
              status={mapStatus(lead.status)}
              sourceName={lead.source_name}
              archivedAgo={getTimeAgo(lead.archived_at ?? lead.updated_at)}
              unarchiving={pendingId === lead.id}
              onUnarchive={handleUnarchive}
            />
          ))}
        </div>
      </div>
    </section>
  );
}
