"use client";

import { useState, useEffect } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, Clock, Archive, ArrowRightLeft, Send } from "lucide-react";
import { ProspectSuggestionBanner } from "@/components/leads/ProspectSuggestionBanner";
import { api, Lead, Message, Qualification, Draft } from "@/lib/api";
import { getTimeAgo, getInitials } from "@/components/inbox/helpers";
import { QualificationCard } from "@/components/inbox/QualificationCard";
import { ConversationThread } from "@/components/inbox/ConversationThread";
import { DraftSidebar } from "@/components/inbox/DraftSidebar";

export default function LeadDetailPage() {
  const params = useParams<{ leadId: string }>();
  const leadId = params.leadId;

  const [lead, setLead] = useState<Lead | null>(null);
  const [qualification, setQualification] = useState<Qualification | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [qualLoading, setQualLoading] = useState(true);
  const [draftLoading, setDraftLoading] = useState(true);

  useEffect(() => {
    if (!leadId) return;
    let cancelled = false;

    async function fetchData() {
      try { const leadData = await api.getLead(leadId); if (!cancelled) setLead(leadData); }
      catch { if (!cancelled) setError(true); }
      try { const msgs = await api.getMessages(leadId); if (!cancelled) setMessages(msgs); } catch {}
      try { const qual = await api.getQualification(leadId); if (!cancelled) setQualification(qual); } catch {}
      if (!cancelled) setQualLoading(false);
      try { const d = await api.getDraft(leadId); if (!cancelled) { setDraft(d); } } catch {}
      if (!cancelled) { setDraftLoading(false); setLoading(false); }
    }

    fetchData();
    const interval = setInterval(() => {
      api.getMessages(leadId).then(setMessages).catch(() => {});
      api.getQualification(leadId).then(setQualification).catch(() => {});
    }, 5000);
    return () => { cancelled = true; clearInterval(interval); };
  }, [leadId]);

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
              <div className="mt-3 flex gap-4">
                <span className="flex items-center gap-1.5 rounded-full bg-[#eff4ff] px-3 py-1 text-xs text-[#737686]"><Clock className="size-3.5" />{getTimeAgo(lead.updated_at)} назад</span>
              </div>
            </div>
          </div>
          <div className="flex gap-3">
            <button className="rounded-lg border border-[#c3c6d7]/30 bg-white px-4 py-2 text-sm font-semibold text-[#0d1c2e] transition-colors hover:bg-[#eff4ff]"><Archive className="mr-1.5 inline size-4" />Архив</button>
            <button className="rounded-lg border border-[#c3c6d7]/30 bg-white px-4 py-2 text-sm font-semibold text-[#0d1c2e] transition-colors hover:bg-[#eff4ff]"><ArrowRightLeft className="mr-1.5 inline size-4" />Передать</button>
          </div>
        </section>

        <ProspectSuggestionBanner leadId={leadId} onChanged={() => { api.getLead(leadId).then(setLead).catch(() => {}); }} />
        <QualificationCard qualification={qualification} loading={qualLoading} />

        <section className="max-w-4xl">
          <ConversationThread messages={messages} initials={initials} />
        </section>
      </div>

      <DraftSidebar leadId={leadId} draft={draft} draftLoading={draftLoading}
        onDraftChanged={setDraft} onMessagesSent={setMessages} />
    </div>
  );
}
