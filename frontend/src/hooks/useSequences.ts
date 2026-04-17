import { useState, useEffect, useCallback } from "react";
import { api, type Sequence } from "@/lib/api";

export function useSequences() {
  const [loading, setLoading] = useState(true);
  const [sequences, setSequences] = useState<Sequence[]>([]);
  const [selectedSeqId, setSelectedSeqId] = useState<string | null>(null);

  useEffect(() => {
    api
      .getSequences()
      .then((data) => {
        setSequences(data);
        if (data.length > 0) setSelectedSeqId(data[0].id);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  const createSequence = useCallback(async (name: string) => {
    const newSeq = await api.createSequence(name);
    setSequences((prev) => [...prev, newSeq]);
    setSelectedSeqId(newSeq.id);
  }, []);

  const deleteSequence = useCallback(
    async (seqId: string) => {
      await api.deleteSequence(seqId);
      setSequences((prev) => prev.filter((s) => s.id !== seqId));
      if (selectedSeqId === seqId) setSelectedSeqId(null);
    },
    [selectedSeqId]
  );

  const toggleSequence = useCallback(async (seqId: string, isActive: boolean) => {
    await api.toggleSequence(seqId, isActive);
    setSequences((prev) => prev.map((s) => (s.id === seqId ? { ...s, is_active: isActive } : s)));
  }, []);

  const renameSequence = useCallback(async (seqId: string, name: string) => {
    const updated = await api.updateSequence(seqId, name);
    setSequences((prev) => prev.map((s) => (s.id === seqId ? { ...s, name: updated.name } : s)));
  }, []);

  const selectedSequence = sequences.find((s) => s.id === selectedSeqId) ?? null;

  return {
    loading,
    sequences,
    selectedSeqId,
    setSelectedSeqId,
    selectedSequence,
    createSequence,
    deleteSequence,
    toggleSequence,
    renameSequence,
  };
}
