import { useState } from "react";
import { Plus, Mail, MessageCircle, Phone, Clock, Sparkles } from "lucide-react";

interface AddStepFormProps {
  onAdd: (params: { channel: "email" | "telegram"; delay_days: number; prompt_hint: string }) => Promise<void>;
  onCancel: () => void;
}

export function AddStepForm({ onAdd, onCancel }: AddStepFormProps) {
  const [channel, setChannel] = useState<"email" | "telegram">("email");
  const [delay, setDelay] = useState(0);
  const [hint, setHint] = useState("первое касание");

  const handleAdd = async () => {
    await onAdd({ channel, delay_days: delay, prompt_hint: hint || "первое касание" });
  };

  return (
    <div className="rounded-xl border border-slate-200 bg-[#eff4ff]/50 p-4">
      <p className="mb-2 text-xs font-semibold text-[#434655]">Канал</p>
      <div className="mb-4 grid grid-cols-3 gap-2">
        <button
          onClick={() => setChannel("email")}
          className={`flex flex-col items-center gap-1.5 rounded-xl border-2 p-3 transition ${
            channel === "email"
              ? "border-[#004ac6] bg-blue-50"
              : "border-slate-200 bg-white hover:border-[#004ac6]/30"
          }`}
        >
          <Mail className={`size-5 ${channel === "email" ? "text-[#004ac6]" : "text-[#434655]"}`} />
          <span className={`text-xs font-medium ${channel === "email" ? "text-[#004ac6]" : "text-[#434655]"}`}>
            Email
          </span>
        </button>
        <button
          onClick={() => setChannel("telegram")}
          className={`flex flex-col items-center gap-1.5 rounded-xl border-2 p-3 transition ${
            channel === "telegram"
              ? "border-[#3e3fcc] bg-purple-50"
              : "border-slate-200 bg-white hover:border-[#3e3fcc]/30"
          }`}
        >
          <MessageCircle className={`size-5 ${channel === "telegram" ? "text-[#3e3fcc]" : "text-[#434655]"}`} />
          <span className={`text-xs font-medium ${channel === "telegram" ? "text-[#3e3fcc]" : "text-[#434655]"}`}>
            Telegram
          </span>
        </button>
        <div className="relative flex flex-col items-center gap-1.5 rounded-xl border-2 border-slate-200 bg-slate-50 p-3 opacity-50 cursor-not-allowed">
          <Phone className="size-5 text-[#434655]" />
          <span className="text-xs font-medium text-[#434655]">Звонок</span>
          <span className="absolute -top-2 -right-2 rounded-full bg-orange-100 px-1.5 py-0.5 text-[9px] font-bold text-orange-600">
            Скоро
          </span>
        </div>
      </div>

      <div className="mb-3">
        <label className="mb-1 flex items-center gap-1.5 text-xs font-semibold text-[#434655]">
          <Clock className="size-3.5" />
          Задержка (дней)
        </label>
        <input
          type="number"
          min={0}
          value={delay}
          onChange={(e) => setDelay(Math.max(0, parseInt(e.target.value) || 0))}
          className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-[#0d1c2e] focus:border-[#004ac6] focus:outline-none focus:ring-2 focus:ring-[#004ac6]/20"
        />
      </div>

      <div className="mb-4">
        <label className="mb-1 flex items-center gap-1.5 text-xs font-semibold text-[#434655]">
          <Sparkles className="size-3.5" />
          Подсказка для AI
        </label>
        <input
          type="text"
          value={hint}
          onChange={(e) => setHint(e.target.value)}
          placeholder="первое касание"
          className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-[#0d1c2e] placeholder:text-[#737686] focus:border-[#004ac6] focus:outline-none focus:ring-2 focus:ring-[#004ac6]/20"
        />
      </div>

      <div className="flex items-center gap-2">
        <button
          onClick={handleAdd}
          className="flex items-center gap-1.5 rounded-lg bg-[#004ac6] px-4 py-2 text-xs font-semibold text-white transition hover:bg-[#004ac6]/90"
        >
          <Plus className="size-3.5" />
          Добавить
        </button>
        <button
          onClick={onCancel}
          className="rounded-lg border border-slate-200 bg-white px-4 py-2 text-xs font-medium text-[#434655] transition hover:bg-slate-50"
        >
          Отмена
        </button>
      </div>
    </div>
  );
}
