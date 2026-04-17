import { useState, useEffect, useCallback } from "react";
import { api, type SourceStatItem } from "@/lib/api";
import { mapProspects, type UIProspect } from "@/components/prospects/constants";

const PER_PAGE = 15;

export function useProspectsPage() {
  const [prospects, setProspects] = useState<UIProspect[]>([]);
  const [searchQuery, setSearchQuery] = useState("");
  const [sourceFilter, setSourceFilter] = useState("");
  const [loading, setLoading] = useState(false);
  const [verifying, setVerifying] = useState(false);
  const [toast, setToast] = useState<{ message: string; type: "success" | "error" } | null>(null);
  const [sourceStats, setSourceStats] = useState<SourceStatItem[]>([]);
  const [page, setPage] = useState(1);

  const showToast = useCallback((message: string, type: "success" | "error") => {
    setToast({ message, type });
    setTimeout(() => setToast(null), 3500);
  }, []);

  const fetchProspects = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.getProspects();
      setProspects(mapProspects(data));
    } catch { /* keep current */ } finally { setLoading(false); }
  }, []);

  useEffect(() => {
    fetchProspects();
    api.getSourceStats().then(setSourceStats).catch(() => {});
  }, [fetchProspects]);

  useEffect(() => { setPage(1); }, [searchQuery, sourceFilter]);

  const handleVerifyBatch = useCallback(async () => {
    setVerifying(true);
    try {
      const [result] = await Promise.all([api.verifyBatch(), new Promise((r) => setTimeout(r, 2500))]);
      await fetchProspects();
      const verified = (result as { verified?: number })?.verified ?? 0;
      showToast(verified > 0 ? `Проверено ${verified} проспектов` : "Нет проспектов для проверки", verified > 0 ? "success" : "error");
    } catch { showToast("Ошибка проверки", "error"); } finally { setVerifying(false); }
  }, [fetchProspects, showToast]);

  const sourceNames = [...new Set(prospects.map((p) => p.sourceName).filter(Boolean))].sort();

  const filteredProspects = prospects.filter((p) => {
    if (sourceFilter && p.sourceName !== sourceFilter) return false;
    if (searchQuery) {
      const q = searchQuery.toLowerCase();
      return p.name.toLowerCase().includes(q) || p.company.toLowerCase().includes(q) || p.email.toLowerCase().includes(q) || p.position.toLowerCase().includes(q);
    }
    return true;
  });

  const totalPages = Math.max(1, Math.ceil(filteredProspects.length / PER_PAGE));
  const safePage = Math.min(page, totalPages);
  const pagedProspects = filteredProspects.slice((safePage - 1) * PER_PAGE, safePage * PER_PAGE);
  const rangeStart = (safePage - 1) * PER_PAGE + 1;
  const rangeEnd = Math.min(safePage * PER_PAGE, filteredProspects.length);

  return {
    prospects,
    searchQuery,
    setSearchQuery,
    sourceFilter,
    setSourceFilter,
    loading,
    verifying,
    toast,
    setToast,
    sourceStats,
    page: safePage,
    setPage,
    showToast,
    fetchProspects,
    handleVerifyBatch,
    sourceNames,
    filteredProspects,
    totalPages,
    pagedProspects,
    rangeStart,
    rangeEnd,
  };
}
