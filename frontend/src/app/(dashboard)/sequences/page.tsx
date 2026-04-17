"use client";

import { useState } from "react";
import { Plus, X } from "lucide-react";
import type { Sequence } from "@/lib/api";
import { useSequences } from "@/hooks/useSequences";
import { useSequenceSteps } from "@/hooks/useSequenceSteps";
import { useProspects } from "@/hooks/useProspects";
import { ConfirmDialog } from "@/components/sequences/ConfirmDialog";
import { EditNameDialog } from "@/components/sequences/EditNameDialog";
import { SequenceList } from "@/components/sequences/SequenceList";
import { AiTipCard } from "@/components/sequences/AiTipCard";
import { StepTimeline } from "@/components/sequences/StepTimeline";
import { ProspectSelector } from "@/components/sequences/ProspectSelector";
import { StatsCard } from "@/components/sequences/StatsCard";

export default function SequencesPage() {
  const seq = useSequences();
  const { steps, stepsLoading, addStep, deleteStep } = useSequenceSteps(seq.selectedSeqId);
  const prosp = useProspects();

  // New sequence inline form
  const [showNewSeq, setShowNewSeq] = useState(false);
  const [newSeqName, setNewSeqName] = useState("");

  // Dialog state (cross-cutting)
  const [confirmDialog, setConfirmDialog] = useState<{ title: string; message: string; onConfirm: () => void } | null>(null);
  const [editDialog, setEditDialog] = useState<{ seqId: string; currentName: string } | null>(null);

  const handleCreateSequence = async () => {
    if (!newSeqName.trim()) return;
    try {
      await seq.createSequence(newSeqName.trim());
      setNewSeqName("");
      setShowNewSeq(false);
    } catch { /* ignore */ }
  };

  const openConfirmDelete = (title: string, message: string, onConfirm: () => void) => {
    setConfirmDialog({ title, message, onConfirm: () => { onConfirm(); setConfirmDialog(null); } });
  };

  return (
    <div className="min-h-screen bg-[#f8f9ff]">
      {/* Header */}
      <header className="px-8 pt-8 pb-6">
        <div className="flex items-start justify-between">
          <div>
            <h1 className="text-2xl sm:text-3xl font-extrabold tracking-tight text-[#0d1c2e]">Секвенции</h1>
            <p className="mt-2 text-[#434655]">Автоматические цепочки холодных писем для вашего B2B отдела</p>
          </div>
          {!showNewSeq ? (
            <button
              onClick={() => setShowNewSeq(true)}
              className="flex items-center gap-2 rounded-xl bg-gradient-to-r from-[#004ac6] to-[#2563eb] px-5 py-2.5 text-sm font-semibold text-white shadow-lg shadow-[#004ac6]/25 transition hover:shadow-xl hover:shadow-[#004ac6]/30"
            >
              <Plus className="size-4" />
              Новая секвенция
            </button>
          ) : (
            <div className="flex items-center gap-2">
              <input
                type="text"
                autoFocus
                value={newSeqName}
                onChange={(e) => setNewSeqName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") handleCreateSequence();
                  if (e.key === "Escape") { setShowNewSeq(false); setNewSeqName(""); }
                }}
                placeholder="Название секвенции..."
                className="rounded-xl border border-slate-200 bg-white px-4 py-2.5 text-sm text-[#0d1c2e] placeholder:text-[#737686] focus:border-[#004ac6] focus:outline-none focus:ring-2 focus:ring-[#004ac6]/20"
              />
              <button
                onClick={handleCreateSequence}
                disabled={!newSeqName.trim()}
                className="flex items-center gap-1.5 rounded-xl bg-gradient-to-r from-[#004ac6] to-[#2563eb] px-4 py-2.5 text-sm font-semibold text-white shadow-lg shadow-[#004ac6]/25 transition hover:shadow-xl disabled:opacity-50"
              >
                <Plus className="size-4" />
                Создать
              </button>
              <button
                onClick={() => { setShowNewSeq(false); setNewSeqName(""); }}
                className="rounded-xl border border-slate-200 bg-white px-3 py-2.5 text-sm text-[#434655] transition hover:bg-slate-50"
              >
                <X className="size-4" />
              </button>
            </div>
          )}
        </div>
        {seq.loading && (
          <div className="mt-3 size-5 animate-spin rounded-full border-2 border-[#004ac6] border-t-transparent" />
        )}
      </header>

      {/* Bento Grid */}
      <div className="grid grid-cols-12 gap-6 px-8 pb-8">
        {/* Left: campaigns + AI tip */}
        <div className="col-span-4 flex flex-col gap-5">
          <SequenceList
            loading={seq.loading}
            sequences={seq.sequences}
            selectedSeqId={seq.selectedSeqId}
            onSelect={seq.setSelectedSeqId}
            onToggle={seq.toggleSequence}
            onEdit={(s: Sequence) => setEditDialog({ seqId: s.id, currentName: s.name })}
            onDelete={(s: Sequence) =>
              openConfirmDelete(
                "Удалить секвенцию",
                `Вы уверены что хотите удалить "${s.name}"? Это действие нельзя отменить.`,
                () => seq.deleteSequence(s.id)
              )
            }
          />
          <AiTipCard sequenceCount={seq.sequences.length} selectedSeqId={seq.selectedSeqId} steps={steps} />
        </div>

        {/* Middle: step timeline */}
        <div className="col-span-5">
          <StepTimeline
            selectedSeqId={seq.selectedSeqId}
            selectedSequenceName={seq.selectedSequence?.name ?? null}
            steps={steps}
            stepsLoading={stepsLoading}
            onDeleteStep={deleteStep}
            onAddStep={addStep}
            onConfirmDelete={openConfirmDelete}
          />
        </div>

        {/* Right: prospects + stats */}
        <div className="col-span-3 flex flex-col gap-5">
          <ProspectSelector
            prospects={prosp.prospects}
            selectedProspects={prosp.selectedProspects}
            selectedSeqId={seq.selectedSeqId}
            launching={prosp.launching}
            launchResult={prosp.launchResult}
            newProspectsCount={prosp.newProspectsCount}
            onToggle={prosp.toggleProspect}
            onSelectAll={prosp.selectAllProspects}
            onLaunch={(ids, sendNow) => seq.selectedSeqId && prosp.launchSequence(seq.selectedSeqId, ids, sendNow)}
            onLaunchAllNew={(sendNow) => {
              if (!seq.selectedSeqId) return;
              const newIds = prosp.prospects.filter((p) => p.status === "new").map((p) => p.id);
              if (newIds.length > 0) prosp.launchSequence(seq.selectedSeqId, newIds, sendNow);
            }}
          />
          <StatsCard />
        </div>
      </div>

      {/* Dialogs */}
      {confirmDialog && (
        <ConfirmDialog
          title={confirmDialog.title}
          message={confirmDialog.message}
          onConfirm={confirmDialog.onConfirm}
          onCancel={() => setConfirmDialog(null)}
        />
      )}
      {editDialog && (
        <EditNameDialog
          currentName={editDialog.currentName}
          onSave={async (name) => {
            try {
              await seq.renameSequence(editDialog.seqId, name);
            } catch { /* ignore */ }
            setEditDialog(null);
          }}
          onCancel={() => setEditDialog(null)}
        />
      )}
    </div>
  );
}
