import { Layers } from "lucide-react";
import type { Sequence } from "@/lib/api";
import { SequenceCard } from "./SequenceCard";

interface SequenceListProps {
  loading: boolean;
  sequences: Sequence[];
  selectedSeqId: string | null;
  onSelect: (id: string) => void;
  onToggle: (id: string, isActive: boolean) => void;
  onEdit: (seq: Sequence) => void;
  onDelete: (seq: Sequence) => void;
}

export function SequenceList({
  loading,
  sequences,
  selectedSeqId,
  onSelect,
  onToggle,
  onEdit,
  onDelete,
}: SequenceListProps) {
  return (
    <>
      <h2 className="flex items-center gap-2 text-sm font-semibold uppercase tracking-wider text-[#434655]">
        <Layers className="size-4" />
        Ваши кампании
      </h2>

      {!loading && sequences.length === 0 && (
        <div className="rounded-2xl border border-dashed border-slate-300 bg-white p-8 text-center">
          <Layers className="mx-auto mb-3 size-8 text-[#c3c6d7]" />
          <p className="text-sm font-medium text-[#434655]">Нет секвенций</p>
          <p className="mt-1 text-xs text-[#737686]">Создайте первую секвенцию, нажав кнопку выше</p>
        </div>
      )}

      {sequences.map((seq) => (
        <SequenceCard
          key={seq.id}
          sequence={seq}
          isSelected={seq.id === selectedSeqId}
          onSelect={() => onSelect(seq.id)}
          onToggle={(isActive) => onToggle(seq.id, isActive)}
          onEdit={() => onEdit(seq)}
          onDelete={() => onDelete(seq)}
        />
      ))}
    </>
  );
}
