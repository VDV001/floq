import { TrendingUp } from "lucide-react";

export function StatsCard() {
  return (
    <div className="rounded-2xl bg-[#004ac6] p-5 text-white shadow-lg">
      <p className="text-xs font-medium uppercase tracking-wider text-white/70">
        Эффективность
      </p>
      <div className="mt-2 flex items-baseline gap-2">
        <span className="text-3xl font-bold">—</span>
        <TrendingUp className="size-5 text-green-300" />
      </div>
      <p className="mt-2 text-xs leading-relaxed text-white/70">
        Средний показатель открытий за текущую секвенцию
      </p>
    </div>
  );
}
