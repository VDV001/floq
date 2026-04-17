import { useState } from "react";
import { api } from "@/lib/api";

interface StepPreviewProps {
  channel: string;
  promptHint: string;
  onClose: () => void;
}

export function StepPreview({ channel, promptHint, onClose }: StepPreviewProps) {
  const [name, setName] = useState("Иван Петров");
  const [text, setText] = useState("");
  const [loading, setLoading] = useState(false);

  const handleGenerate = async () => {
    setLoading(true);
    try {
      const res = await api.previewMessage(name, "", "", channel, promptHint);
      setText(res.text);
    } catch {
      setText("Ошибка генерации");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="mt-3 rounded-xl border border-[#004ac6]/10 bg-[#eff4ff]/50 p-4">
      {!text ? (
        <div className="space-y-3">
          <label className="block text-xs font-semibold text-[#434655]">Имя проспекта для примера</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Иван Петров"
            className="w-full rounded-lg border-none bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-[#004ac6]/20"
          />
          <div className="flex gap-2">
            <button
              disabled={loading || !name}
              onClick={handleGenerate}
              className="rounded-lg bg-[#004ac6] px-4 py-2 text-xs font-bold text-white disabled:opacity-50"
            >
              {loading ? "Генерация..." : "Сгенерировать"}
            </button>
            <button
              onClick={onClose}
              className="rounded-lg border border-[#c3c6d7] px-4 py-2 text-xs font-bold text-[#434655]"
            >
              Отмена
            </button>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          <p className="text-xs font-semibold text-[#434655]">Пример сообщения:</p>
          <div className="rounded-lg bg-white p-3 text-sm leading-relaxed text-[#0d1c2e] whitespace-pre-wrap">
            {text}
          </div>
          <button onClick={onClose} className="text-xs font-bold text-[#004ac6] hover:underline">
            Закрыть
          </button>
        </div>
      )}
    </div>
  );
}
