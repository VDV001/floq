import { useState } from "react";
import { Send, RefreshCw, Zap } from "lucide-react";
import { Switch } from "@/components/ui/switch";
import { api } from "@/lib/api";
import type { Draft, Message } from "@/lib/api";

interface DraftSidebarProps {
  leadId: string;
  draft: Draft | null;
  draftLoading: boolean;
  onDraftChanged: (d: Draft | null) => void;
  onMessagesSent: (msgs: Message[]) => void;
}

export function DraftSidebar({ leadId, draft, draftLoading, onDraftChanged, onMessagesSent }: DraftSidebarProps) {
  const [draftText, setDraftText] = useState(draft?.body || "");
  const [regenerating, setRegenerating] = useState(false);
  const [sending, setSending] = useState(false);

  // Reset the editor when navigating to another lead so a half-written reply
  // never leaks across leads (textarea is always editable now).
  const [prevLeadId, setPrevLeadId] = useState(leadId);
  if (leadId !== prevLeadId) {
    setPrevLeadId(leadId);
    setDraftText(draft?.body || "");
  }

  // Sync when draft changes externally
  if (draft && draftText === "" && draft.body) setDraftText(draft.body);

  return (
    <aside className="flex w-96 shrink-0 flex-col border-l border-[#c3c6d7]/10 bg-white p-6">
      <div className="mb-4 flex items-center justify-between">
        <h4 className="text-sm font-bold text-[#0d1c2e]">Черновик ответа</h4>
        {draft && (
          <div className="flex items-center gap-1 rounded-full bg-[#e1e0ff] px-2 py-1 text-[0.6rem] font-bold uppercase text-[#3e3fcc]">
            <Zap className="size-3" /> Умный черновик
          </div>
        )}
      </div>

      <div className="relative mb-4 flex-1">
        <div className="h-full rounded-xl border border-[#c3c6d7]/20 bg-[#eff4ff] p-4">
          {draftLoading ? (
            <div className="flex h-full items-center justify-center">
              <div className="size-5 animate-spin rounded-full border-2 border-[#3e3fcc] border-t-transparent" />
            </div>
          ) : (
            <textarea className="h-full w-full resize-none border-none bg-transparent text-sm leading-relaxed text-[#0d1c2e] outline-none placeholder:italic placeholder:text-[#434655]"
              value={draftText} onChange={(e) => setDraftText(e.target.value)} spellCheck={false}
              placeholder="Напишите ответ вручную или сгенерируйте черновик кнопкой ниже." />
          )}
        </div>
      </div>

      <div className="space-y-3">
        <button onClick={async () => {
          setRegenerating(true);
          try { const d = await api.regenerateDraft(leadId); onDraftChanged(d); setDraftText(d.body); }
          catch { alert("Ошибка генерации черновика"); }
          finally { setRegenerating(false); }
        }} disabled={regenerating}
          className="flex w-full items-center justify-center gap-2 rounded-xl border border-[#c3c6d7]/30 py-3 text-sm font-bold text-[#0d1c2e] transition-all hover:bg-[#eff4ff] disabled:opacity-50">
          {regenerating && <RefreshCw className="size-4 animate-spin" />}
          {regenerating ? "Генерация..." : draft ? "Перегенерировать" : "Сгенерировать черновик ИИ"}
        </button>
        <button onClick={async () => {
          if (!draftText.trim()) return;
          setSending(true);
          try {
            await api.sendMessage(leadId, draftText);
            const msgs = await api.getMessages(leadId);
            onMessagesSent(msgs);
            setDraftText("");
            onDraftChanged(null);
          } catch { alert("Ошибка отправки"); }
          finally { setSending(false); }
        }} disabled={!draftText.trim() || sending}
          className="flex w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-r from-[#004ac6] to-[#2563eb] py-4 text-sm font-bold text-white shadow-lg shadow-[#004ac6]/20 transition-all hover:opacity-90 active:scale-95 disabled:opacity-50">
          {sending ? <RefreshCw className="size-4 animate-spin" /> : <Send className="size-4" />}
          Отправить ответ
        </button>
      </div>

      <div className="mt-8 border-t border-[#c3c6d7]/10 pt-8">
        <p className="mb-4 text-[0.65rem] font-bold uppercase text-[#434655]">Настройки автоматизации</p>
        <div className="flex items-center justify-between py-2">
          <span className="text-xs font-medium text-[#0d1c2e]">Авто-фоллоуапы</span>
          <Switch defaultChecked />
        </div>
        <div className="flex items-center justify-between py-2">
          <span className="text-xs font-medium text-[#0d1c2e]">Согласование черновиков</span>
          <Switch defaultChecked />
        </div>
      </div>
    </aside>
  );
}
