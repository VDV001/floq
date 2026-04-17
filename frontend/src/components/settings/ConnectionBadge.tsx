import { Check } from "lucide-react";

export function ConnectionBadge({ active }: { active: boolean }) {
  return active ? (
    <span className="flex items-center gap-1 rounded-full bg-green-100 px-3 py-1 text-[10px] font-bold uppercase tracking-wider text-green-700">
      <Check className="size-3" />
      Подключен
    </span>
  ) : (
    <span className="rounded-full bg-[#ba1a1a]/10 px-3 py-1 text-[10px] font-bold uppercase tracking-wider text-[#ba1a1a]">
      Не подключен
    </span>
  );
}
