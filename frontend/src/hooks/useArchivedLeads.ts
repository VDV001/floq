"use client";

import { useEffect, useState } from "react";
import { api, Lead } from "@/lib/api";

// useArchivedLeads loads the archive-view feed (only archived leads, newest
// first). It owns the list so the page can drop a row optimistically the
// moment an unarchive succeeds — an unarchived lead belongs back in the
// working inbox, not in this view.
export function useArchivedLeads() {
  const [loading, setLoading] = useState(true);
  const [leads, setLeads] = useState<Lead[]>([]);
  // error distinguishes a genuinely empty archive from a failed load — without
  // it a 500/offline both render as "Архив пуст", hiding leads that exist but
  // could not be fetched. The page reads this to show a load-error state.
  const [error, setError] = useState(false);

  useEffect(() => {
    let cancelled = false;
    api
      .getArchivedLeads()
      .then((data) => {
        if (!cancelled) setLeads(data);
      })
      .catch(() => {
        if (!cancelled) setError(true);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // removeLead drops a lead from the local list after it has been unarchived.
  const removeLead = (id: string) =>
    setLeads((prev) => prev.filter((l) => l.id !== id));

  return { loading, leads, error, removeLead };
}
