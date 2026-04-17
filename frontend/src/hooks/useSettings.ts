import { useState, useEffect } from "react";
import { api, type UserSettings } from "@/lib/api";

export type TestResult = { success: boolean; message?: string; error?: string } | null;

export const PROVIDER_DEFAULTS: Record<string, string> = {
  ollama: "gemma3:4b",
  claude: "claude-sonnet-4-20250514",
  openai: "gpt-4o",
  groq: "openai/gpt-oss-120b",
};

export function useSettingsCore() {
  const [settings, setSettings] = useState<UserSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saveResult, setSaveResult] = useState<"success" | "error" | null>(null);

  useEffect(() => {
    api.getSettings().then(setSettings).catch(() => {}).finally(() => setLoading(false));
  }, []);

  const save = async (update: Partial<UserSettings>) => {
    setSaving(true);
    setSaveResult(null);
    try {
      const updated = await api.updateSettings(update);
      setSettings(updated);
      setSaveResult("success");
      setTimeout(() => setSaveResult(null), 4000);
    } catch {
      setSaveResult("error");
      setTimeout(() => setSaveResult(null), 4000);
    } finally {
      setSaving(false);
    }
  };

  return { settings, setSettings, loading, saving, setSaving, saveResult, save };
}

export function useTelegramBot(settings: UserSettings | null, setSettings: (s: UserSettings) => void) {
  const [tgToken, setTgToken] = useState("");
  const [saving, setSaving] = useState(false);

  const connect = async () => {
    if (!tgToken || tgToken.startsWith("...")) return;
    setSaving(true);
    try {
      const updated = await api.updateSettings({ telegram_bot_token: tgToken });
      setSettings(updated);
      setTgToken("");
    } catch { alert("Не удалось подключить бота. Проверьте токен."); }
    finally { setSaving(false); }
  };

  return { tgToken, setTgToken, saving, connect, maskedToken: settings?.telegram_bot_token || "", botActive: !!settings?.telegram_bot_active };
}

export function useTelegramAccount() {
  const [phone, setPhone] = useState("");
  const [code, setCode] = useState("");
  const [codeHash, setCodeHash] = useState("");
  const [step, setStep] = useState<"idle" | "code_sent" | "connected">("idle");
  const [connectedPhone, setConnectedPhone] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    api.tgAccountStatus().then((st) => {
      if (st.connected) { setStep("connected"); setConnectedPhone(st.phone); }
    }).catch(() => {});
  }, []);

  const sendCode = async () => {
    const clean = phone.replace(/[\s()-]/g, "");
    if (!clean.startsWith("+") || clean.replace(/\D/g, "").length < 10) { setError("Введите номер в формате +7XXXXXXXXXX"); return; }
    setLoading(true); setError("");
    try {
      const res = await api.tgAccountSendCode(clean);
      setCodeHash(res.code_hash); setPhone(clean); setStep("code_sent");
    } catch (err) {
      const msg = err instanceof Error ? err.message : "";
      if (msg.includes("PHONE_NUMBER_INVALID") || msg.includes("400")) setError("Неверный номер телефона. Проверьте формат.");
      else if (msg.includes("PHONE_NUMBER_FLOOD")) setError("Слишком много попыток. Подождите 10 минут.");
      else setError("Не удалось отправить код. Попробуйте позже.");
    } finally { setLoading(false); }
  };

  const verify = async () => {
    setLoading(true); setError("");
    try {
      await api.tgAccountVerify(phone, code, codeHash);
      setStep("connected"); setConnectedPhone(phone); setCode("");
    } catch (err) {
      const msg = err instanceof Error ? err.message : "";
      if (msg.includes("PHONE_CODE_INVALID") || msg.includes("400")) setError("Неверный код. Проверьте и попробуйте ещё раз.");
      else if (msg.includes("PHONE_CODE_EXPIRED")) { setError("Код истёк. Запросите новый."); setStep("idle"); }
      else if (msg.includes("SESSION_PASSWORD_NEEDED")) setError("Аккаунт защищён 2FA паролем. Пока не поддерживается.");
      else setError("Ошибка авторизации. Попробуйте позже.");
    } finally { setLoading(false); }
  };

  const disconnect = async () => {
    try { await api.tgAccountDisconnect(); setStep("idle"); setConnectedPhone(""); setPhone(""); }
    catch { setError("Ошибка отключения"); }
  };

  const reset = () => { setStep("idle"); setCode(""); setError(""); };

  return { phone, setPhone, code, setCode, step, connectedPhone, loading, error, setError, sendCode, verify, disconnect, reset };
}

export function useImapSettings(settings: UserSettings | null) {
  const [host, setHost] = useState("");
  const [port, setPort] = useState("993");
  const [user, setUser] = useState("");
  const [password, setPassword] = useState("");
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<TestResult>(null);
  const [verified, setVerified] = useState<boolean | null>(null);

  useEffect(() => {
    if (!settings) return;
    setHost(settings.imap_host); setPort(settings.imap_port); setUser(settings.imap_user);
  }, [settings]);

  const test = async () => {
    setTesting(true); setTestResult(null);
    try {
      const h = host || settings?.imap_host || "";
      const p = port || settings?.imap_port || "993";
      const u = user || settings?.imap_user || "";
      const pw = password && !password.startsWith("...") ? password : "";
      if (!h || !u) { setTestResult({ success: false, error: "Заполните хост и пользователя IMAP" }); return; }
      const res = await api.testIMAP(h, p, u, pw || "", !pw);
      setTestResult(res); setVerified(res.success);
    } catch { setTestResult({ success: false, error: "Ошибка запроса" }); setVerified(false); }
    finally { setTesting(false); }
  };

  const active = verified ?? !!settings?.imap_active;

  return { host, setHost, port, setPort, user, setUser, password, setPassword, testing, testResult, setTestResult, test, active, maskedPassword: settings?.imap_password || "" };
}

export function useResendSettings(settings: UserSettings | null, setSettings: (s: UserSettings | null) => void) {
  const [key, setKey] = useState("");
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<TestResult>(null);
  const [verified, setVerified] = useState<boolean | null>(null);

  const test = async () => {
    setTesting(true); setTestResult(null);
    try {
      const k = key && !key.startsWith("...") ? key : "";
      const res = await api.testResend(k, !k);
      setTestResult(res); setVerified(res.success);
      if (res.success && k) { const updated = await api.updateSettings({ resend_api_key: k }); setSettings(updated); setKey(""); }
    } catch { setTestResult({ success: false, error: "Ошибка запроса" }); setVerified(false); }
    finally { setTesting(false); }
  };

  const active = verified ?? !!settings?.resend_active;

  return { key, setKey, testing, testResult, setTestResult, test, active, maskedKey: settings?.resend_api_key || "", hasKey: !!settings?.resend_api_key };
}

export function useSmtpSettings(settings: UserSettings | null, setSettings: (s: UserSettings) => void) {
  const [host, setHost] = useState("");
  const [port, setPort] = useState("465");
  const [user, setUser] = useState("");
  const [password, setPassword] = useState("");
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<TestResult>(null);
  const [verified, setVerified] = useState<boolean | null>(null);

  useEffect(() => {
    if (!settings) return;
    setHost(settings.smtp_host); setPort(settings.smtp_port || "465"); setUser(settings.smtp_user);
  }, [settings]);

  const test = async () => {
    setTesting(true); setTestResult(null);
    try {
      const h = host || settings?.smtp_host || "";
      const p = port || settings?.smtp_port || "465";
      const u = user || settings?.smtp_user || "";
      const pw = password && !password.startsWith("...") ? password : "";
      if (!h || !u) { setTestResult({ success: false, error: "Заполните хост и пользователя SMTP" }); return; }
      const res = await api.testSMTP(h, p, u, pw);
      setTestResult(res); setVerified(res.success);
      if (res.success && (u || pw)) {
        const update: Partial<UserSettings> = { smtp_host: h, smtp_port: p, smtp_user: u };
        if (pw) update.smtp_password = pw;
        const updated = await api.updateSettings(update); setSettings(updated); setPassword("");
      }
    } catch { setTestResult({ success: false, error: "Ошибка запроса" }); setVerified(false); }
    finally { setTesting(false); }
  };

  const active = verified ?? !!settings?.smtp_active;

  return { host, setHost, port, setPort, user, setUser, password, setPassword, testing, testResult, setTestResult, test, active, maskedPassword: settings?.smtp_password || "" };
}

export function useAiSettings(settings: UserSettings | null, setSettings: (s: UserSettings) => void) {
  const [provider, setProvider] = useState("ollama");
  const [model, setModel] = useState("gemma3:4b");
  const [apiKey, setApiKey] = useState("");
  const [showKey, setShowKey] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<TestResult>(null);
  const [verified, setVerified] = useState<boolean | null>(null);

  useEffect(() => {
    if (!settings) return;
    setProvider(settings.ai_provider || "ollama");
    setModel(settings.ai_model || PROVIDER_DEFAULTS[settings.ai_provider] || "gemma3:4b");
  }, [settings]);

  const test = async () => {
    setTesting(true); setTestResult(null);
    try {
      const k = apiKey && !apiKey.startsWith("...") ? apiKey : "";
      const res = await api.testAI(provider, model, k, !k);
      setTestResult(res); setVerified(res.success);
      if (res.success) {
        const update: Partial<typeof settings & object> = { ai_provider: provider, ai_model: model };
        if (k) update.ai_api_key = k;
        const updated = await api.updateSettings(update); setSettings(updated);
        if (k) setApiKey("");
      }
    } catch { setTestResult({ success: false, error: "Ошибка запроса" }); setVerified(false); }
    finally { setTesting(false); }
  };

  const active = verified ?? !!settings?.ai_active;

  return { provider, setProvider, model, setModel, apiKey, setApiKey, showKey, setShowKey, testing, testResult, setTestResult, test, active, maskedKey: settings?.ai_api_key || "", hasKey: !!settings?.ai_api_key };
}
