import { Webhook, Loader2, Trash2, Send } from "lucide-react";
import type { WebhookEndpoint } from "@/lib/api";
import { ConnectionBadge } from "./ConnectionBadge";
import { HintIcon } from "./HintIcon";

export type WebhookNotice = { ok: boolean; message: string } | null;

interface WebhookSectionProps {
  endpoints: WebhookEndpoint[];
  eventTypes: string[];
  loading: boolean;
  url: string;
  setUrl: (v: string) => void;
  secret: string;
  setSecret: (v: string) => void;
  selectedEvents: string[];
  toggleEvent: (e: string) => void;
  creating: boolean;
  createError: string | null;
  onCreate: () => void;
  onDelete: (id: string) => void;
  onTest: (id: string) => void;
  testingId: string | null;
  notice: WebhookNotice;
}

export function WebhookSection({
  endpoints, eventTypes, loading,
  url, setUrl, secret, setSecret, selectedEvents, toggleEvent,
  creating, createError, onCreate, onDelete, onTest, testingId, notice,
}: WebhookSectionProps) {
  const canCreate = url.trim() !== "" && secret.trim().length >= 16 && selectedEvents.length > 0 && !creating;

  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Webhook className="size-5 text-[#0d1c2e]" />
          <h4 className="font-bold text-[#0d1c2e]">Вебхуки</h4>
          <HintIcon text="Floq отправляет POST-запрос на ваш URL при событиях (новый лид, квалификация и т.д.) — для интеграции с CRM/Zapier/n8n. Каждый запрос подписан HMAC в заголовке X-Floq-Signature." />
        </div>
        <ConnectionBadge active={endpoints.length > 0} />
      </div>

      {notice && (
        <div
          role={notice.ok ? "status" : "alert"}
          className={`mb-4 rounded-lg px-4 py-2.5 text-sm font-medium ${
            notice.ok ? "bg-green-50 text-green-700" : "bg-red-50 text-red-600"
          }`}
        >
          {notice.message}
        </div>
      )}

      {/* Existing endpoints */}
      {loading ? (
        <div className="flex items-center gap-2 py-4 text-sm text-[#434655]/70">
          <Loader2 className="size-4 animate-spin" /> Загрузка…
        </div>
      ) : endpoints.length === 0 ? (
        <p className="mb-5 text-sm text-[#434655]/70">Пока нет ни одного вебхука. Добавьте первый ниже.</p>
      ) : (
        <ul className="mb-6 space-y-3">
          {endpoints.map((ep) => (
            <li key={ep.id} className="rounded-xl bg-[#f7f9ff] p-4 ring-1 ring-[#c3c6d7]/10">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="truncate font-mono text-sm font-bold text-[#0d1c2e]">{ep.url}</p>
                  <div className="mt-2 flex flex-wrap gap-1.5">
                    {ep.events.map((e) => (
                      <span key={e} className="rounded-full bg-[#dbe1ff] px-2.5 py-0.5 text-[11px] font-bold text-[#004ac6]">
                        {e}
                      </span>
                    ))}
                  </div>
                </div>
                <div className="flex shrink-0 gap-2">
                  <button
                    onClick={() => onTest(ep.id)}
                    disabled={testingId === ep.id}
                    aria-label={`Проверить ${ep.url}`}
                    className="flex items-center gap-1.5 rounded-lg bg-[#eff4ff] px-3 py-2 text-xs font-bold text-[#004ac6] transition-colors hover:bg-[#dbe1ff] disabled:opacity-50"
                  >
                    {testingId === ep.id ? <Loader2 className="size-3.5 animate-spin" /> : <Send className="size-3.5" />}
                    Проверить
                  </button>
                  <button
                    onClick={() => onDelete(ep.id)}
                    aria-label={`Удалить ${ep.url}`}
                    className="flex items-center gap-1.5 rounded-lg bg-red-50 px-3 py-2 text-xs font-bold text-red-600 transition-colors hover:bg-red-100"
                  >
                    <Trash2 className="size-3.5" /> Удалить
                  </button>
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}

      {/* Add new endpoint */}
      <div className="space-y-4 rounded-xl border border-dashed border-[#c3c6d7]/40 p-4">
        <div className="text-xs font-bold uppercase tracking-wider text-[#434655]/60">Новый вебхук</div>
        <input
          type="url"
          placeholder="https://hooks.zapier.com/…"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm placeholder-[#434655]/50 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
        />
        <input
          type="password"
          placeholder="Секрет для подписи (≥16 символов)"
          value={secret}
          onChange={(e) => setSecret(e.target.value)}
          className="w-full rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm placeholder-[#434655]/50 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
        />
        <fieldset>
          <legend className="mb-2 text-xs font-bold text-[#434655]/70">События</legend>
          <div className="flex flex-wrap gap-x-5 gap-y-2">
            {eventTypes.map((e) => (
              <label key={e} className="flex cursor-pointer items-center gap-2 text-sm text-[#0d1c2e]">
                <input
                  type="checkbox"
                  checked={selectedEvents.includes(e)}
                  onChange={() => toggleEvent(e)}
                  className="size-4 rounded border-[#c3c6d7] text-[#004ac6] focus:ring-[#004ac6]/30"
                />
                <span className="font-mono text-xs">{e}</span>
              </label>
            ))}
          </div>
        </fieldset>

        {createError && (
          <p role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm font-medium text-red-600">
            {createError}
          </p>
        )}

        <button
          onClick={onCreate}
          disabled={!canCreate}
          className="flex items-center justify-center gap-2 rounded-lg bg-gradient-to-br from-[#004ac6] to-[#2563eb] px-6 py-3 text-sm font-bold text-white transition-all hover:brightness-110 disabled:opacity-50"
        >
          {creating ? <Loader2 className="size-[18px] animate-spin" /> : "Добавить вебхук"}
        </button>
      </div>
    </div>
  );
}
