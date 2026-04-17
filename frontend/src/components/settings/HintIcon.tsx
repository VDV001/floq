import { Info } from "lucide-react";

export function HintIcon({ text }: { text: string }) {
  return (
    <span className="group/hint relative">
      <Info className="size-4 cursor-help text-[#434655]/30 transition-colors hover:text-[#004ac6]" />
      <span className="pointer-events-none absolute left-full top-1/2 z-50 ml-2 w-64 -translate-y-1/2 rounded-lg bg-[#0d1c2e] px-4 py-3 text-xs font-normal leading-relaxed text-white/90 opacity-0 shadow-xl transition-opacity duration-200 group-hover/hint:pointer-events-auto group-hover/hint:opacity-100">
        {text}
        <span className="absolute right-full top-1/2 -translate-y-1/2 border-4 border-transparent border-r-[#0d1c2e]" />
      </span>
    </span>
  );
}
