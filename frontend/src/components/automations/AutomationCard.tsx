import { Switch } from "@/components/ui/switch";
import type { Automation } from "./constants";

interface AutomationCardProps {
  auto: Automation;
  isOn: boolean;
  inputValue: number | undefined;
  onToggle: () => void;
  onInputChange: (val: number) => void;
}

export function AutomationCard({ auto, isOn, inputValue, onToggle, onInputChange }: AutomationCardProps) {
  const Icon = auto.icon;

  return (
    <div
      className="group rounded-xl border border-[#e5e7eb] bg-white p-6 transition-all duration-300 hover:shadow-xl hover:shadow-[#004ac6]/5"
    >
      {/* Top: icon + toggle */}
      <div className="mb-6 flex items-start justify-between">
        <div
          className={`flex size-12 items-center justify-center rounded-lg ${auto.iconBg}`}
        >
          <Icon className={`size-6 ${auto.iconColor}`} />
        </div>
        <Switch checked={isOn} onCheckedChange={onToggle} />
      </div>

      {/* Title + description */}
      <h3 className="mb-2 text-lg font-bold text-[#0d1c2e]">
        {auto.title}
      </h3>
      <p className="text-sm leading-relaxed text-[#434655]">
        {auto.description}
      </p>

      {/* Bottom area */}
      {auto.bottom.type === "tag" ? (
        <div className="mt-6 flex items-center gap-2 border-t border-[#c3c6d7]/10 pt-4 text-xs font-semibold">
          {(() => {
            const TagIcon = auto.bottom.icon;
            return (
              <>
                <TagIcon className={`size-3.5 ${auto.bottom.color}`} />
                <span className={auto.bottom.color}>
                  {auto.bottom.text}
                </span>
              </>
            );
          })()}
        </div>
      ) : (
        <div className="mt-4 flex flex-col gap-2 rounded-lg bg-[#f8f9ff] p-3">
          <label className="text-[10px] font-bold uppercase tracking-wider text-[#434655]">
            {auto.bottom.label}
          </label>
          <input
            type="number"
            value={inputValue ?? auto.bottom.defaultValue}
            onChange={(e) => onInputChange(Number(e.target.value))}
            className="rounded border border-[#c3c6d7]/30 bg-white px-2 py-1 text-sm outline-none focus:border-[#004ac6] focus:ring-1 focus:ring-[#004ac6]"
          />
        </div>
      )}
    </div>
  );
}
