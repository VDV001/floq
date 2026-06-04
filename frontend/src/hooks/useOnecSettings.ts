"use client";

import { useCallback, useEffect, useState } from "react";
import {
  api,
  type OnecAuthType,
  type OnecConfig,
  type OnecConfigUpdate,
  type OnecMappingRule,
  type OnecTestResult,
} from "@/lib/api";

export type OnecTestState = OnecTestResult | null;
export type SaveState = "success" | "error" | null;

// isMaskedOrEmpty reports whether a secret field holds the masked placeholder
// (or nothing) — i.e. the user never typed a replacement, so the backend should
// keep the stored secret. Masked values come back as "...xxxx".
function isMaskedOrEmpty(v: string): boolean {
  return v === "" || v.startsWith("...");
}

const EMPTY_RULE: OnecMappingRule = { external_type: "", kind: "payment", email_field: "" };

// useOnecSettings drives the /settings 1C section. Unlike the channel settings
// sub-hooks it owns its own persistence (the 1C config is a separate endpoint,
// not /api/settings). Secrets are never echoed back into the form — the form
// holds only what the operator types; the masked stored value lives in `config`.
export function useOnecSettings() {
  const [config, setConfig] = useState<OnecConfig | null>(null);
  const [loading, setLoading] = useState(true);

  // Config form state (secrets start blank; the masked value is shown as a
  // placeholder, never as a value).
  const [baseURL, setBaseURL] = useState("");
  const [authType, setAuthType] = useState<OnecAuthType>("basic");
  const [authSecret, setAuthSecret] = useState("");
  const [isActive, setIsActive] = useState(false);

  const [saving, setSaving] = useState(false);
  const [saveResult, setSaveResult] = useState<SaveState>(null);

  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<OnecTestState>(null);

  const [regenerating, setRegenerating] = useState(false);
  const [fullWebhook, setFullWebhook] = useState<string | null>(null);

  // Mapping state.
  const [rules, setRules] = useState<OnecMappingRule[]>([]);
  const [savingMapping, setSavingMapping] = useState(false);
  const [mappingResult, setMappingResult] = useState<OnecTestState>(null);

  const applyConfig = useCallback((c: OnecConfig) => {
    setConfig(c);
    setBaseURL(c.base_url);
    setAuthType(c.auth_type);
    setIsActive(c.is_active);
    setAuthSecret(""); // never seed the secret input
  }, []);

  useEffect(() => {
    let alive = true;
    Promise.all([api.getOnecConfig(), api.getOnecMapping()])
      .then(([c, m]) => {
        if (!alive) return;
        applyConfig(c);
        setRules(m.rules);
      })
      .catch(() => {})
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, [applyConfig]);

  const save = useCallback(async () => {
    setSaving(true);
    setSaveResult(null);
    try {
      const update: OnecConfigUpdate = { base_url: baseURL, auth_type: authType, is_active: isActive };
      if (!isMaskedOrEmpty(authSecret)) {
        update.auth_secret = authSecret;
      }
      const updated = await api.updateOnecConfig(update);
      applyConfig(updated);
      setSaveResult("success");
    } catch {
      setSaveResult("error");
    } finally {
      setSaving(false);
    }
  }, [baseURL, authType, isActive, authSecret, applyConfig]);

  const test = useCallback(async () => {
    setTesting(true);
    setTestResult(null);
    try {
      const payload: { base_url?: string; auth_type?: string; auth_secret?: string } = {
        base_url: baseURL || config?.base_url || "",
        auth_type: authType,
      };
      if (!isMaskedOrEmpty(authSecret)) {
        payload.auth_secret = authSecret;
      }
      setTestResult(await api.testOnec(payload));
    } catch {
      setTestResult({ success: false, error: "Ошибка запроса" });
    } finally {
      setTesting(false);
    }
  }, [baseURL, authType, authSecret, config]);

  const regenerateWebhook = useCallback(async () => {
    setRegenerating(true);
    try {
      const { webhook_secret } = await api.regenerateOnecWebhook();
      setFullWebhook(webhook_secret);
      // Refresh so the masked webhook tail updates too.
      const c = await api.getOnecConfig();
      applyConfig(c);
    } catch {
      // leave fullWebhook null; the section surfaces no secret on failure
    } finally {
      setRegenerating(false);
    }
  }, [applyConfig]);

  const addRule = useCallback(() => setRules((rs) => [...rs, { ...EMPTY_RULE }]), []);
  const updateRule = useCallback(
    (index: number, patch: Partial<OnecMappingRule>) =>
      setRules((rs) => rs.map((r, i) => (i === index ? { ...r, ...patch } : r))),
    [],
  );
  const removeRule = useCallback((index: number) => setRules((rs) => rs.filter((_, i) => i !== index)), []);

  const saveMapping = useCallback(async () => {
    setSavingMapping(true);
    setMappingResult(null);
    try {
      await api.updateOnecMapping(rules);
      setMappingResult({ success: true });
    } catch (err) {
      setMappingResult({ success: false, error: err instanceof Error ? err.message : "Ошибка сохранения" });
    } finally {
      setSavingMapping(false);
    }
  }, [rules]);

  return {
    loading,
    config,
    // config form
    baseURL,
    setBaseURL,
    authType,
    setAuthType,
    authSecret,
    setAuthSecret,
    isActive,
    setIsActive,
    maskedSecret: config?.auth_secret ?? "",
    maskedWebhook: config?.webhook_secret ?? "",
    fullWebhook,
    saving,
    saveResult,
    save,
    testing,
    testResult,
    setTestResult,
    test,
    regenerating,
    regenerateWebhook,
    // mapping
    rules,
    addRule,
    updateRule,
    removeRule,
    savingMapping,
    mappingResult,
    setMappingResult,
    saveMapping,
  };
}
