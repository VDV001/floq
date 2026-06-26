"use client";

import { useState, useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, Clock, Archive, ArchiveRestore, Send } from "lucide-react";
import { useNotify } from "@/components/notifications/NotificationProvider";
import { ProspectSuggestionBanner } from "@/components/leads/ProspectSuggestionBanner";
import { PendingReplySection } from "@/components/leads/PendingReplySection";
import { IdentityBadge } from "@/components/leads/IdentityBadge";
import { api, Lead, Message, Qualification, Draft, Enrichment } from "@/lib/api";
import { unarchiveLead } from "@/lib/leadActions";
import { getTimeAgo, getInitials } from "@/components/inbox/helpers";
import { QualificationCard } from "@/components/inbox/QualificationCard";
import { EnrichmentCard } from "@/components/inbox/EnrichmentCard";
import { ConversationThread } from "@/components/inbox/ConversationThread";
import { DraftSidebar } from "@/components/inbox/DraftSidebar";

export default function LeadDetailPage() {
  const params = useParams<{ leadId: string }>();
  const leadId = params.leadId;
  const router = useRouter();
  const { notify, notifyError } = useNotify();
  const [archiving, setArchiving] = useState(false);
  const [confirmingArchive, setConfirmingArchive] = useState(false);
  const [unarchiving, setUnarchiving] = useState(false);

  const [lead, setLead] = useState<Lead | null>(null);
  const [qualification, setQualification] = useState<Qualification | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [enrichment, setEnrichment] = useState<Enrichment | null>(null);
  const [enrichmentLoading, setEnrichmentLoading] = useState(true);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [qualLoading, setQualLoading] = useState(true);
  const [draftLoading, setDraftLoading] = useState(true);
  // Aggregated view is on by default — matches the server-side
  // user_settings.aggregated_inbox_view DEFAULT TRUE. The hydrate
  // effect below upgrades the value from /api/settings before the
  // messages fetch runs, so we never load a stale per-source thread
  // when the user has opted in to aggregation (or vice versa).
  const [aggregated, setAggregated] = useState<boolean | null>(null);

  // Hydrate the inbox-view preference once. The second effect waits
  // for `aggregated` to leave its `null` sentinel before fetching,
  // so each user gets exactly one initial messages request — no
  // duplicate-fetch flicker on the first mount.
  useEffect(() => {
    let cancelled = false;
    api
      .getSettings()
      .then((s) => {
        if (!cancelled) setAggregated(s.aggregated_inbox_view);
      })
      .catch(() => {
        if (!cancelled) setAggregated(true); // server default
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!leadId || aggregated === null) return;
    let cancelled = false;
    const flag = aggregated;

    async function fetchData() {
      let email: string | undefined;
      try { const leadData = await api.getLead(leadId); if (!cancelled) setLead(leadData); email = leadData.email_address; }
      catch { if (!cancelled) setError(true); }
      if (email) {
        try { const enr = await api.getEnrichment(email); if (!cancelled) setEnrichment(enr); } catch {}
      }
      if (!cancelled) setEnrichmentLoading(false);
      try { const msgs = await api.getMessages(leadId, { aggregated: flag }); if (!cancelled) setMessages(msgs); } catch {}
      try { const qual = await api.getQualification(leadId); if (!cancelled) setQualification(qual); } catch {}
      if (!cancelled) setQualLoading(false);
      try { const d = await api.getDraft(leadId); if (!cancelled) { setDraft(d); } } catch {}
      if (!cancelled) { setDraftLoading(false); setLoading(false); }
    }

    fetchData();
    const interval = setInterval(() => {
      api.getMessages(leadId, { aggregated: flag }).then(setMessages).catch(() => {});
      api.getQualification(leadId).then(setQualification).catch(() => {});
    }, 5000);
    return () => { cancelled = true; clearInterval(interval); };
  }, [leadId, aggregated]);

  async function handleArchive() {
    if (archiving) return;
    setArchiving(true);
    try {
      await api.archiveLead(leadId);
      notify({ type: "success", title: "Лид в архиве", message: "Он скрыт из ленты и аналитики." });
      router.push("/inbox");
    } catch (err) {
      notifyError(err, "Не удалось архивировать лид");
      setArchiving(false);
    }
  }

  async function handleUnarchive() {
    if (unarchiving) return;
    setUnarchiving(true);
    const ok = await unarchiveLead(leadId, notify, notifyError);
    // Clear the flag in place so the page flips back to the archive control
    // without a full refetch — the lead is active again.
    if (ok) setLead((prev) => (prev ? { ...prev, archived_at: undefined } : prev));
    setUnarchiving(false);
  }

  if (loading) return <div className="flex h-full items-center justify-center"><div className="size-8 animate-spin rounded-full border-4 border-[#3b6ef6] border-t-transparent" /></div>;
  if (error || !lead) return (
    <div className="flex h-full items-center justify-center"><div className="text-center">
      <p className="text-2xl font-bold text-[#0d1c2e]">Лид не найден</p>
      <Link href="/inbox" className="mt-4 inline-flex items-center gap-1.5 text-sm text-[#004ac6] hover:underline"><ArrowLeft className="size-4" />Назад к лидам</Link>
    </div></div>
  );

  const initials = getInitials(lead.contact_name);

  return (
    <div className="flex h-full overflow-hidden">
      <div className="flex-1 overflow-y-auto px-4 sm:px-6 lg:px-10 py-8">
        <Link href="/inbox" className="mb-6 inline-flex items-center gap-1.5 text-sm text-[#434655] transition-colors hover:text-[#004ac6]">
          <ArrowLeft className="size-4" /> Назад
        </Link>

        {/* Contact Info */}
        <section className="mb-10 flex items-start justify-between">
          <div className="flex items-center gap-6">
            <div className="flex size-20 shrink-0 items-center justify-center rounded-2xl bg-[#dbe1ff] text-2xl font-bold text-[#004ac6] shadow-sm">{initials}</div>
            <div>
              <div className="mb-1 flex items-center gap-3">
                <h2 className="text-xl sm:text-2xl lg:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">{lead.contact_name}</h2>
                {lead.channel === "telegram" && <span className="flex size-6 items-center justify-center rounded-md bg-[#0088cc] text-white"><Send className="size-3.5" /></span>}
              </div>
              <p className="font-medium text-[#434655]">{lead.company ? <>в <span className="font-bold text-[#004ac6]">{lead.company}</span></> : "—"}</p>
              <div className="mt-3 flex flex-wrap items-center gap-2">
                <span className="flex items-center gap-1.5 rounded-full bg-[#eff4ff] px-3 py-1 text-xs text-[#737686]"><Clock className="size-3.5" />{getTimeAgo(lead.updated_at)} назад</span>
                <IdentityBadge identity={lead.identity} currentLeadId={leadId} />
              </div>
            </div>
          </div>
          <div className="flex gap-3">
            {lead.archived_at ? (
              <button onClick={handleUnarchive} disabled={unarchiving} className="rounded-lg border border-[#c3c6d7]/30 bg-white px-4 py-2 text-sm font-semibold text-[#0d1c2e] transition-colors hover:bg-[#eff4ff] disabled:cursor-not-allowed disabled:opacity-60"><ArchiveRestore className="mr-1.5 inline size-4" />{unarchiving ? "Возвращаем…" : "Разархивировать"}</button>
            ) : confirmingArchive ? (
              <div className="flex items-center gap-2">
                <span className="text-sm text-[#434655]">Архивировать лид?</span>
                <button onClick={handleArchive} disabled={archiving} className="rounded-lg bg-[#0d1c2e] px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-[#0d1c2e]/90 disabled:cursor-not-allowed disabled:opacity-60">{archiving ? "Архивируем…" : "Да, в архив"}</button>
                <button onClick={() => setConfirmingArchive(false)} disabled={archiving} className="rounded-lg border border-[#c3c6d7]/30 bg-white px-4 py-2 text-sm font-semibold text-[#0d1c2e] transition-colors hover:bg-[#eff4ff] disabled:opacity-60">Отмена</button>
              </div>
            ) : (
              <button onClick={() => setConfirmingArchive(true)} className="rounded-lg border border-[#c3c6d7]/30 bg-white px-4 py-2 text-sm font-semibold text-[#0d1c2e] transition-colors hover:bg-[#eff4ff]"><Archive className="mr-1.5 inline size-4" />Архив</button>
            )}
          </div>
        </section>

        <ProspectSuggestionBanner leadId={leadId} onChanged={() => { api.getLead(leadId).then(setLead).catch(() => {}); }} />
        <PendingReplySection
          leadId={leadId}
          onApproved={() => {
            api.getMessages(leadId, { aggregated: aggregated ?? true }).then(setMessages).catch(() => {});
          }}
        />
        <QualificationCard qualification={qualification} loading={qualLoading} />
        {lead.email_address && <EnrichmentCard enrichment={enrichment} loading={enrichmentLoading} />}

        <section className="max-w-4xl">
          <ConversationThread messages={messages} initials={initials} />
        </section>
      </div>

      <DraftSidebar leadId={leadId} draft={draft} draftLoading={draftLoading}
        onDraftChanged={setDraft} onMessagesSent={setMessages} />
    </div>
  );
}
