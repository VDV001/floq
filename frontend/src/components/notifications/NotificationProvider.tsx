"use client";

import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { CheckCircle2, AlertTriangle, Info, X } from "lucide-react";
import { ApiError } from "@/lib/api";

export type NotificationType = "success" | "error" | "info";

export interface NotificationAction {
  label: string;
  href: string;
}

export interface NotificationInput {
  type: NotificationType;
  title: string;
  message: string;
  /** "What to do" — a human, actionable next step shown under the message. */
  remedy?: string;
  action?: NotificationAction;
}

interface Notification extends NotificationInput {
  id: number;
}

interface NotifyApi {
  notify: (n: NotificationInput) => void;
  /** Map a thrown error (ApiError or generic) to a human error notification. */
  notifyError: (err: unknown, fallbackTitle?: string) => void;
  dismiss: (id: number) => void;
  notifications: Notification[];
}

const NotificationContext = createContext<NotifyApi | null>(null);

// Errors stay until dismissed (the user must read the cause + remedy);
// success/info auto-dismiss after a few seconds.
const AUTO_DISMISS_MS: Record<NotificationType, number | null> = {
  success: 4000,
  info: 5000,
  error: null,
};

// Known backend error codes → a direct link to the page that fixes them.
const CODE_ACTIONS: Record<string, NotificationAction> = {
  ai_not_configured: { label: "Открыть настройки ИИ", href: "/settings" },
  smtp_not_configured: { label: "Открыть настройки почты", href: "/settings" },
  resend_not_configured: { label: "Открыть настройки почты", href: "/settings" },
};

export function NotificationProvider({ children }: { children: React.ReactNode }) {
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const nextId = useRef(1);
  const timers = useRef(new Map<number, ReturnType<typeof setTimeout>>());

  const dismiss = useCallback((id: number) => {
    const timer = timers.current.get(id);
    if (timer) {
      clearTimeout(timer);
      timers.current.delete(id);
    }
    setNotifications((list) => list.filter((n) => n.id !== id));
  }, []);

  const notify = useCallback(
    (input: NotificationInput) => {
      const id = nextId.current++;
      setNotifications((list) => [...list, { ...input, id }]);
      const ttl = AUTO_DISMISS_MS[input.type];
      if (ttl !== null) {
        timers.current.set(id, setTimeout(() => dismiss(id), ttl));
      }
    },
    [dismiss],
  );

  // Clear any pending auto-dismiss timers when the provider unmounts.
  useEffect(() => {
    const pending = timers.current;
    return () => {
      pending.forEach((t) => clearTimeout(t));
      pending.clear();
    };
  }, []);

  const notifyError = useCallback(
    (err: unknown, fallbackTitle = "Не удалось выполнить") => {
      if (err instanceof ApiError) {
        notify({
          type: "error",
          title: fallbackTitle,
          message: err.message,
          remedy: err.remedy,
          action: err.code ? CODE_ACTIONS[err.code] : undefined,
        });
        return;
      }
      const message = err instanceof Error ? err.message : "Неизвестная ошибка";
      notify({ type: "error", title: fallbackTitle, message });
    },
    [notify],
  );

  return (
    <NotificationContext.Provider value={{ notify, notifyError, dismiss, notifications }}>
      {children}
      <NotificationViewport notifications={notifications} onDismiss={dismiss} />
    </NotificationContext.Provider>
  );
}

const ICONS: Record<NotificationType, typeof CheckCircle2> = {
  success: CheckCircle2,
  error: AlertTriangle,
  info: Info,
};

const STYLES: Record<NotificationType, string> = {
  success: "border-emerald-200 bg-white",
  error: "border-[#ffb4ab] bg-[#fff8f7]",
  info: "border-[#c3c6d7]/30 bg-white",
};

const ICON_COLORS: Record<NotificationType, string> = {
  success: "text-emerald-500",
  error: "text-[#ba1a1a]",
  info: "text-[#004ac6]",
};

function NotificationViewport({
  notifications,
  onDismiss,
}: {
  notifications: Notification[];
  onDismiss: (id: number) => void;
}) {
  if (notifications.length === 0) return null;
  return (
    <div className="fixed bottom-6 right-6 z-[100] flex w-[min(92vw,380px)] flex-col gap-3" role="region" aria-label="Уведомления">
      {notifications.map((n) => {
        const Icon = ICONS[n.type];
        return (
          <div
            key={n.id}
            role={n.type === "error" ? "alert" : "status"}
            className={`flex gap-3 rounded-xl border p-4 shadow-lg ${STYLES[n.type]}`}
          >
            <Icon className={`mt-0.5 size-5 shrink-0 ${ICON_COLORS[n.type]}`} />
            <div className="min-w-0 flex-1">
              <p className="text-sm font-bold text-[#0d1c2e]">{n.title}</p>
              <p className="mt-0.5 text-sm text-[#434655] break-words">{n.message}</p>
              {n.remedy && (
                <p className="mt-1.5 text-xs font-medium text-[#434655]">
                  <span className="font-bold">Что делать: </span>
                  {n.remedy}
                </p>
              )}
              {n.action && (
                <Link
                  href={n.action.href}
                  onClick={() => onDismiss(n.id)}
                  className="mt-2 inline-block text-xs font-bold text-[#004ac6] hover:underline"
                >
                  {n.action.label} →
                </Link>
              )}
            </div>
            <button
              type="button"
              onClick={() => onDismiss(n.id)}
              aria-label="Закрыть уведомление"
              className="shrink-0 text-slate-400 transition-colors hover:text-[#0d1c2e]"
            >
              <X className="size-4" />
            </button>
          </div>
        );
      })}
    </div>
  );
}

export function useNotify(): NotifyApi {
  const ctx = useContext(NotificationContext);
  if (!ctx) {
    throw new Error("useNotify must be used within a NotificationProvider");
  }
  return ctx;
}
