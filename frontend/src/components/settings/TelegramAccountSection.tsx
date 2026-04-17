import { Send } from "lucide-react";
import { ConnectionBadge } from "./ConnectionBadge";
import { HintIcon } from "./HintIcon";

interface TelegramAccountSectionProps {
  step: "idle" | "code_sent" | "connected";
  connectedPhone: string;
  phone: string;
  setPhone: (v: string) => void;
  code: string;
  setCode: (v: string) => void;
  loading: boolean;
  error: string;
  setError: (v: string) => void;
  onSendCode: () => void;
  onVerify: () => void;
  onDisconnect: () => void;
  onReset: () => void;
}

export function TelegramAccountSection({
  step, connectedPhone, phone, setPhone, code, setCode,
  loading, error, setError, onSendCode, onVerify, onDisconnect, onReset,
}: TelegramAccountSectionProps) {
  return (
    <div>
      <div className="mb-4 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Send className="size-5 text-[#7c3aed]" />
          <h4 className="font-bold text-[#0d1c2e]">TG аккаунт (рассылка)</h4>
          <HintIcon text="Подключите личный Telegram аккаунт для автоматической рассылки персонализированных сообщений проспектам. Отправка от вашего имени, не от бота." />
        </div>
        <ConnectionBadge active={step === "connected"} />
      </div>

      {step === "connected" ? (
        <div className="flex items-center justify-between rounded-lg bg-green-50 px-4 py-3">
          <p className="text-sm text-green-700">
            Подключен: <span className="font-bold">{connectedPhone}</span>
          </p>
          <button onClick={onDisconnect} className="text-xs font-bold text-red-500 hover:underline">
            Отключить
          </button>
        </div>
      ) : step === "code_sent" ? (
        <div className="space-y-3">
          <div className="rounded-lg bg-[#eff4ff] px-4 py-3">
            <p className="text-xs text-[#434655]">
              Код отправлен на <span className="font-bold">{phone}</span> в Telegram. Проверьте «Избранное» или SMS.
            </p>
          </div>
          <div className="flex gap-3">
            <input
              type="text"
              inputMode="numeric"
              maxLength={6}
              placeholder="Код из Telegram"
              value={code}
              onChange={(e) => { setCode(e.target.value.replace(/\D/g, "")); setError(""); }}
              className={`flex-1 rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm text-center tracking-widest font-mono outline-none focus:ring-2 focus:ring-[#7c3aed]/20 ${error ? "ring-2 ring-red-300" : ""}`}
            />
            <button
              disabled={loading || code.length < 4}
              onClick={onVerify}
              className="rounded-lg bg-[#7c3aed] px-6 py-3 text-sm font-bold text-white hover:brightness-110 disabled:opacity-50"
            >
              {loading ? "..." : "Подтвердить"}
            </button>
          </div>
          <div className="flex items-center justify-between">
            {error && <p className="text-xs text-red-500">{error}</p>}
            <button onClick={onReset} className="ml-auto text-xs text-[#434655] hover:underline">
              Ввести другой номер
            </button>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          <p className="text-xs text-[#434655]/70">Введите номер телефона в международном формате (начинается с +)</p>
          <div className="flex gap-3">
            <input
              type="tel"
              placeholder="+7 999 123 4567"
              value={phone}
              onChange={(e) => {
                let v = e.target.value.replace(/[^\d+\s()-]/g, "");
                if (v && !v.startsWith("+")) v = "+" + v;
                setPhone(v);
                setError("");
              }}
              className={`flex-1 rounded-lg border-none bg-[#eff4ff] px-4 py-3 text-sm placeholder-[#434655]/50 outline-none focus:ring-2 focus:ring-[#7c3aed]/20 ${error ? "ring-2 ring-red-300" : ""}`}
            />
            <button
              disabled={loading || !phone || phone.replace(/\D/g, "").length < 10}
              onClick={onSendCode}
              className="rounded-lg bg-[#7c3aed] px-6 py-3 text-sm font-bold text-white hover:brightness-110 disabled:opacity-50"
            >
              {loading ? "..." : "Отправить код"}
            </button>
          </div>
          {error && <p className="text-xs text-red-500">{error}</p>}
        </div>
      )}
    </div>
  );
}
