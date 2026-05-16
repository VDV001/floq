"use client";

import { Users } from "lucide-react";

interface InboxViewSectionProps {
  aggregated: boolean;
  saving: boolean;
  onToggle: (next: boolean) => void;
}

/**
 * InboxViewSection exposes the `user_settings.aggregated_inbox_view`
 * preference (#27) as a single toggle. When enabled (default), the
 * lead detail page merges messages from every lead sharing the same
 * Identity — useful once the IdentityResolver backfill (PR2) has run.
 */
export function InboxViewSection({ aggregated, saving, onToggle }: InboxViewSectionProps) {
  return (
    <section
      aria-labelledby="inbox-view-heading"
      className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-[#c3c6d7]/10"
    >
      <div className="mb-4 flex items-center gap-3">
        <Users className="size-5 text-[#004ac6]" aria-hidden="true" />
        <h3 id="inbox-view-heading" className="text-lg font-bold text-[#0d1c2e]">
          Вид входящих
        </h3>
      </div>

      <label className="flex cursor-pointer items-start gap-3">
        <input
          type="checkbox"
          checked={aggregated}
          disabled={saving}
          onChange={(e) => onToggle(e.target.checked)}
          className="mt-1 size-4 rounded border-[#c3c6d7] text-[#004ac6] focus:ring-2 focus:ring-[#3b6ef6]"
          aria-describedby="inbox-view-help"
        />
        <span className="text-sm">
          <span className="block font-semibold text-[#0d1c2e]">
            Объединённая лента сообщений по контакту
          </span>
          <span id="inbox-view-help" className="mt-1 block text-xs text-[#737686]">
            Когда включено, на странице лида показываются сообщения изо всех каналов
            (email, Telegram, прошлые лиды), связанных с этим контактом по единой
            идентичности. Выключите, чтобы вернуться к режиму &laquo;один источник = один тред&raquo;.
          </span>
        </span>
      </label>
    </section>
  );
}
