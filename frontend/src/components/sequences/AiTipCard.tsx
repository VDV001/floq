import { Sparkles } from "lucide-react";
import type { SequenceStep } from "@/lib/api";

interface AiTipCardProps {
  sequenceCount: number;
  selectedSeqId: string | null;
  steps: SequenceStep[];
}

export function AiTipCard({ sequenceCount, selectedSeqId, steps }: AiTipCardProps) {
  const handleOptimize = () => {
    if (selectedSeqId && steps.length > 0) {
      const channels = new Set<string>(steps.map((s) => s.channel));
      const allChannels = ["email", "telegram", "phone_call"];
      const missing = allChannels.filter((c) => !channels.has(c));
      if (missing.length > 0) {
        const labels: Record<string, string> = { email: "Email", telegram: "Telegram", phone_call: "Звонок" };
        alert(`Добавьте шаги с каналами: ${missing.map((c) => labels[c] || c).join(", ")}`);
      } else {
        alert("Все каналы уже используются — секвенция оптимальна!");
      }
    } else {
      alert("Выберите секвенцию и добавьте хотя бы один шаг");
    }
  };

  return (
    <div className="rounded-2xl border border-[#3e3fcc]/20 bg-[#e1e0ff]/20 p-5">
      <div className="mb-2 flex items-center gap-2">
        <Sparkles className="size-4 text-[#3e3fcc]" />
        <span className="text-xs font-bold text-[#3e3fcc]">AI Совет</span>
      </div>
      <p className="text-xs leading-relaxed text-[#0d1c2e]/80">
        {sequenceCount === 0
          ? "Создайте первую секвенцию для автоматизации холодного outreach."
          : `У вас ${sequenceCount} секвенций. Добавьте шаги с разными каналами для повышения конверсии.`}
      </p>
      <button
        onClick={handleOptimize}
        className="mt-3 text-xs font-semibold text-[#2f2ebe] hover:underline"
      >
        Оптимизировать сейчас &rarr;
      </button>
    </div>
  );
}
