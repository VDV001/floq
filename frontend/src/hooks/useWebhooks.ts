"use client";

import { useCallback, useEffect, useState } from "react";
import { api, ApiError, type WebhookEndpoint } from "@/lib/api";

export type WebhookNoticeState = { ok: boolean; message: string } | null;

// useWebhooks drives the /settings webhooks section. Like the 1C hook it owns
// its own persistence (the webhook endpoints are a separate CRUD API, not
// /api/settings). The signing secret is write-only: it is sent on create and
// never read back, so the form clears it after a successful add.
export function useWebhooks() {
  const [endpoints, setEndpoints] = useState<WebhookEndpoint[]>([]);
  const [eventTypes, setEventTypes] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);

  // Add-form state.
  const [url, setUrl] = useState("");
  const [secret, setSecret] = useState("");
  const [selectedEvents, setSelectedEvents] = useState<string[]>([]);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  // Per-endpoint action state.
  const [testingId, setTestingId] = useState<string | null>(null);
  const [notice, setNotice] = useState<WebhookNoticeState>(null);

  const reload = useCallback(async () => {
    const list = await api.getWebhooks();
    setEndpoints(list ?? []);
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [list, types] = await Promise.all([api.getWebhooks(), api.getWebhookEventTypes()]);
        if (cancelled) return;
        setEndpoints(list ?? []);
        setEventTypes(types ?? []);
      } catch {
        // A load failure leaves empty lists; the section renders its empty state.
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const toggleEvent = useCallback((e: string) => {
    setSelectedEvents((prev) => (prev.includes(e) ? prev.filter((x) => x !== e) : [...prev, e]));
  }, []);

  const create = useCallback(async () => {
    setCreating(true);
    setCreateError(null);
    try {
      await api.createWebhook({ url: url.trim(), events: selectedEvents, secret });
      // Reset the form (secret is write-only) and reload the list.
      setUrl("");
      setSecret("");
      setSelectedEvents([]);
      await reload();
      setNotice({ ok: true, message: "Вебхук добавлен" });
    } catch (err) {
      setCreateError(err instanceof ApiError ? err.message : "Не удалось добавить вебхук");
    } finally {
      setCreating(false);
    }
  }, [url, secret, selectedEvents, reload]);

  const remove = useCallback(
    async (id: string) => {
      try {
        await api.deleteWebhook(id);
        await reload();
        setNotice({ ok: true, message: "Вебхук удалён" });
      } catch {
        setNotice({ ok: false, message: "Не удалось удалить вебхук" });
      }
    },
    [reload],
  );

  const test = useCallback(async (id: string) => {
    setTestingId(id);
    setNotice(null);
    try {
      await api.testWebhook(id);
      setNotice({ ok: true, message: "Тестовая доставка поставлена в очередь" });
    } catch {
      setNotice({ ok: false, message: "Не удалось отправить тестовую доставку" });
    } finally {
      setTestingId(null);
    }
  }, []);

  return {
    endpoints,
    eventTypes,
    loading,
    url,
    setUrl,
    secret,
    setSecret,
    selectedEvents,
    toggleEvent,
    creating,
    createError,
    create,
    remove,
    test,
    testingId,
    notice,
    setNotice,
  };
}
