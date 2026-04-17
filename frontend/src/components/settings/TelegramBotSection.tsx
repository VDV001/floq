import { Send } from "lucide-react";
import { ConnectionBadge } from "./ConnectionBadge";
import { HintIcon } from "./HintIcon";

interface TelegramBotSectionProps {
  botActive: boolean;
  maskedToken: string;
  tgToken: string;
  setTgToken: (v: string) => void;
  saving: boolean;
  onConnect: () => void;
}

export function TelegramBotSection({ botActive, maskedToken, tgToken, setTgToken, saving, onConnect }: TelegramBotSectionProps) {
  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Send className="size-5 text-[#229ED9]" />
          <h4 className="font-bold text-[#0d1c2e]">Telegram bot</h4>
          <HintIcon text="Бот для приёма входящих сообщений. Клиент пишет боту → Floq создаёт лида → AI квалифицирует и генерирует ответ. Создайте бота через @BotFather в Telegram." />
        </div>
        <ConnectionBadge active={botActive} />
      </div>
      <div className="flex gap-4">
        <input
          type="password"
          placeholder={maskedToken || "Введите токен..."}
          value={tgToken}
          onChange={(e) => setTgToken(e.target.value)}
          className="flex-1 rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm placeholder-[#434655]/50 outline-none transition-all focus:ring-2 focus:ring-[#004ac6]/20"
        />
        <button
          onClick={onConnect}
          disabled={saving || !tgToken || tgToken.startsWith("...")}
          className="rounded-lg bg-[#2563eb] px-6 py-3 text-sm font-bold text-white transition-all hover:brightness-110 disabled:opacity-50"
        >
          {saving ? "..." : "Подключить"}
        </button>
      </div>
    </div>
  );
}
