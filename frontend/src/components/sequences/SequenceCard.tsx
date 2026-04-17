import type { Sequence } from "@/lib/api";
import { Switch } from "@/components/ui/switch";
import { Separator } from "@/components/ui/separator";

interface SequenceCardProps {
  sequence: Sequence;
  isSelected: boolean;
  onSelect: () => void;
  onToggle: (isActive: boolean) => void;
  onEdit: () => void;
  onDelete: () => void;
}

export function SequenceCard({ sequence, isSelected, onSelect, onToggle, onEdit, onDelete }: SequenceCardProps) {
  return (
    <div
      onClick={onSelect}
      className={`cursor-pointer rounded-2xl bg-white p-5 shadow-sm transition ${
        isSelected
          ? "border-2 border-[#22c55e] shadow-md shadow-[#22c55e]/15"
          : "border border-[#e2e8f0] hover:border-[#004ac6]/30"
      } ${!sequence.is_active && !isSelected ? "opacity-70 grayscale" : !sequence.is_active ? "opacity-85" : ""}`}
    >
      <div className="flex items-start justify-between gap-2">
        <h3 className="text-sm font-semibold text-[#0d1c2e]">{sequence.name}</h3>
        <span
          className={`shrink-0 rounded-full px-2.5 py-0.5 text-xs font-medium ${
            sequence.is_active ? "bg-green-100 text-green-700" : "bg-slate-100 text-slate-500"
          }`}
        >
          {sequence.is_active ? "Активна" : "Пауза"}
        </span>
      </div>

      <div className="mt-3 flex items-center gap-2 text-xs text-[#737686]">
        <span>Создана: {new Date(sequence.created_at).toLocaleDateString("ru-RU")}</span>
      </div>

      <Separator className="my-3" />

      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Switch checked={sequence.is_active} onCheckedChange={(checked) => onToggle(checked)} />
          <span className="text-[10px] font-bold uppercase text-[#737686]">
            {sequence.is_active ? "Активна" : "На паузе"}
          </span>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={(e) => { e.stopPropagation(); onEdit(); }}
            className="text-xs font-medium text-[#004ac6] hover:underline"
          >
            Редактировать
          </button>
          <button
            onClick={(e) => { e.stopPropagation(); onDelete(); }}
            className="text-xs font-medium text-red-500 hover:underline"
          >
            Удалить
          </button>
        </div>
      </div>
    </div>
  );
}
