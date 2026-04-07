"use client";

/* eslint-disable @typescript-eslint/no-explicit-any */
declare global {
  interface Window {
    SpeechRecognition: any;
    webkitSpeechRecognition: any;
  }
}

import { useState, useRef, useEffect, useCallback } from "react";
import { usePathname } from "next/navigation";
import {
  Sparkles,
  X,
  Send,
  Trash2,
  Loader2,
  Maximize2,
  Minimize2,
  Mic,
  MicOff,
  Eye,
} from "lucide-react";
import ReactMarkdown from "react-markdown";
import { api } from "@/lib/api";

interface ChatMessage {
  role: "user" | "assistant";
  content: string;
}

function TypingIndicator() {
  return (
    <div className="flex items-center gap-1 px-4 py-3">
      <span className="size-2 animate-bounce rounded-full bg-[#434655]/40 [animation-delay:0ms]" />
      <span className="size-2 animate-bounce rounded-full bg-[#434655]/40 [animation-delay:150ms]" />
      <span className="size-2 animate-bounce rounded-full bg-[#434655]/40 [animation-delay:300ms]" />
    </div>
  );
}

export function FloatingActionButton() {
  const [open, setOpen] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [showContext, setShowContext] = useState(false);
  const [contextData, setContextData] = useState<string | null>(null);
  const [listening, setListening] = useState(false);
  const recognitionRef = useRef<ReturnType<typeof Object> | null>(null);

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const pathname = usePathname();

  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages, loading, scrollToBottom]);

  useEffect(() => {
    if (open) {
      setTimeout(() => inputRef.current?.focus(), 100);
    }
  }, [open]);

  const deriveContext = (): string => {
    if (!pathname) return "";
    const segment = pathname.replace(/^\//, "").split("/")[0];
    return segment || "dashboard";
  };

  const sendMessage = async () => {
    const text = input.trim();
    if (!text || loading) return;

    const userMsg: ChatMessage = { role: "user", content: text };
    const updatedMessages = [...messages, userMsg];
    setMessages(updatedMessages);
    setInput("");
    setLoading(true);

    try {
      const history = updatedMessages.map((m) => ({
        role: m.role,
        content: m.content,
      }));
      const { reply } = await api.chatWithAI(text, history, deriveContext());
      setMessages((prev) => [...prev, { role: "assistant", content: reply }]);
    } catch (err) {
      const errorText =
        err instanceof Error ? err.message : "Произошла ошибка";
      setMessages((prev) => [
        ...prev,
        { role: "assistant", content: `Ошибка: ${errorText}` },
      ]);
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    sendMessage();
  };

  const clearChat = () => {
    setMessages([]);
  };

  // Fetch context data for transparency panel
  const fetchContext = async () => {
    if (showContext) {
      setShowContext(false);
      return;
    }
    try {
      const usage = await api.getUsage();
      const ctx = `Страница: ${deriveContext()}\nЛидов: ${usage.total_leads}\nЛидов в месяце: ${usage.month_leads}\nПлан: ${usage.plan} (${usage.limit} лимит)`;
      setContextData(ctx);
      setShowContext(true);
    } catch {
      setContextData("Не удалось загрузить контекст");
      setShowContext(true);
    }
  };

  // Voice input via Web Speech API
  const toggleVoice = () => {
    if (listening) {
      recognitionRef.current?.stop();
      setListening(false);
      return;
    }

    const SpeechRecognition =
      typeof window !== "undefined"
        ? window.SpeechRecognition || window.webkitSpeechRecognition
        : null;

    if (!SpeechRecognition) {
      alert("Браузер не поддерживает голосовой ввод");
      return;
    }

    const recognition = new SpeechRecognition();
    recognition.lang = "ru-RU";
    recognition.interimResults = false;
    recognition.maxAlternatives = 1;

    recognition.onresult = (event: any) => {
      const transcript = event.results[0][0].transcript;
      setInput((prev) => prev + (prev ? " " : "") + transcript);
      setListening(false);
    };

    recognition.onerror = () => {
      setListening(false);
    };

    recognition.onend = () => {
      setListening(false);
    };

    recognitionRef.current = recognition;
    recognition.start();
    setListening(true);
  };

  // Panel size classes
  const panelClasses = expanded
    ? "fixed inset-y-4 right-4 z-50 flex w-[min(48rem,calc(100vw-2rem))] flex-col rounded-2xl border border-[#c3c6d7]/20 bg-white shadow-2xl"
    : "fixed bottom-24 right-6 z-50 flex h-[min(32rem,calc(100vh-8rem))] w-[calc(100vw-2rem)] flex-col overflow-hidden rounded-2xl border border-[#c3c6d7]/20 bg-white shadow-2xl sm:w-96";

  return (
    <>
      {/* Chat panel */}
      {open && (
        <div className={panelClasses}>
          {/* Header */}
          <div className="flex items-center justify-between px-5 py-4">
            <div className="flex items-center gap-3">
              <Sparkles className="size-6 text-[#3b6ef6]" />
              <div>
                <p className="text-sm font-bold text-[#0d1c2e]">Floq AI</p>
                <p className="text-[11px] text-[#434655]">
                  Ассистент по продажам
                </p>
              </div>
            </div>
            <div className="flex items-center gap-1">
              {/* Context / transparency */}
              <button
                onClick={fetchContext}
                title="Контекст AI"
                className={`rounded-lg p-1.5 transition-colors hover:bg-[#eff4ff] ${
                  showContext ? "bg-[#eff4ff] text-[#3b6ef6]" : "text-[#434655] hover:text-[#0d1c2e]"
                }`}
              >
                <Eye className="size-4" />
              </button>
              {/* Voice input */}
              <button
                onClick={toggleVoice}
                title="Голосовой ввод"
                className={`rounded-lg p-1.5 transition-colors hover:bg-[#eff4ff] ${
                  listening ? "bg-red-50 text-red-500" : "text-[#434655] hover:text-[#0d1c2e]"
                }`}
              >
                {listening ? <MicOff className="size-4" /> : <Mic className="size-4" />}
              </button>
              {/* Expand / collapse */}
              <button
                onClick={() => setExpanded(!expanded)}
                title={expanded ? "Компактный вид" : "Развернуть"}
                className="rounded-lg p-1.5 text-[#434655] transition-colors hover:bg-[#eff4ff] hover:text-[#0d1c2e]"
              >
                {expanded ? <Minimize2 className="size-4" /> : <Maximize2 className="size-4" />}
              </button>
              {/* Clear */}
              {messages.length > 0 && (
                <button
                  onClick={clearChat}
                  title="Очистить чат"
                  className="rounded-lg p-1.5 text-[#434655] transition-colors hover:bg-[#eff4ff] hover:text-[#0d1c2e]"
                >
                  <Trash2 className="size-4" />
                </button>
              )}
            </div>
          </div>

          {/* Context panel */}
          {showContext && contextData && (
            <div className="mx-4 mb-2 rounded-lg bg-[#eff4ff] px-4 py-3">
              <p className="mb-1 text-[10px] font-bold uppercase tracking-wider text-[#004ac6]">
                Что видит AI
              </p>
              <pre className="whitespace-pre-wrap text-xs leading-relaxed text-[#434655]">
                {contextData}
              </pre>
            </div>
          )}

          {/* Messages */}
          <div className="flex-1 overflow-y-auto px-4 py-4">
            {messages.length === 0 && !loading && (
              <div className="flex flex-col items-center justify-center py-12 text-center">
                <div className="mb-4 flex size-14 items-center justify-center rounded-full bg-[#eff4ff]">
                  <Sparkles className="size-7 text-[#3b6ef6]" />
                </div>
                <p className="text-sm font-medium text-[#0d1c2e]">
                  Чем могу помочь?
                </p>
                <p className="mt-1 text-xs text-[#434655]">
                  Спросите про лидов, воронку или аналитику
                </p>
              </div>
            )}

            {messages.map((msg, i) => (
              <div
                key={i}
                className={`mb-3 flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}
              >
                <div
                  className={`rounded-2xl px-4 py-2.5 text-sm leading-relaxed ${
                    expanded ? "max-w-[85%]" : "max-w-[80%]"
                  } ${
                    msg.role === "user"
                      ? "rounded-br-md bg-[#2563eb] text-white whitespace-pre-wrap"
                      : "rounded-bl-md bg-[#eff4ff] text-[#0d1c2e]"
                  }`}
                >
                  {msg.role === "user" ? (
                    msg.content
                  ) : (
                    <div className="prose prose-sm max-w-none prose-headings:text-[#0d1c2e] prose-headings:text-sm prose-headings:font-bold prose-p:my-1 prose-ul:my-1 prose-li:my-0 prose-table:text-xs prose-th:px-2 prose-th:py-1 prose-td:px-2 prose-td:py-1 prose-strong:text-[#0d1c2e]">
                      <ReactMarkdown>{msg.content}</ReactMarkdown>
                    </div>
                  )}
                </div>
              </div>
            ))}

            {loading && (
              <div className="mb-3 flex justify-start">
                <div className="rounded-2xl rounded-bl-md bg-[#eff4ff]">
                  <TypingIndicator />
                </div>
              </div>
            )}

            <div ref={messagesEndRef} />
          </div>

          {/* Input */}
          <form onSubmit={handleSubmit} className={expanded ? "px-5 py-4" : "p-4"}>
            <div className="flex items-center gap-3">
              <input
                ref={inputRef}
                type="text"
                value={input}
                onChange={(e) => setInput(e.target.value)}
                placeholder={listening ? "Говорите..." : "Спросите что-нибудь..."}
                disabled={loading}
                className={`flex-1 rounded-xl bg-[#eff4ff] px-4 text-[#0d1c2e] placeholder-[#737686] outline-none transition-colors focus:ring-2 focus:ring-[#3b6ef6]/30 disabled:opacity-50 ${
                  expanded ? "py-3 text-sm" : "py-2.5 text-sm"
                } ${listening ? "ring-2 ring-red-300" : ""}`}
              />
              <button
                type="submit"
                disabled={loading || !input.trim()}
                className={`flex items-center justify-center rounded-xl bg-[#3b6ef6] text-white transition-colors hover:bg-[#2563eb] disabled:opacity-50 ${
                  expanded ? "size-11" : "size-10"
                }`}
              >
                {loading ? (
                  <Loader2 className="size-4 animate-spin" />
                ) : (
                  <Send className="size-4" />
                )}
              </button>
            </div>
          </form>
        </div>
      )}

      {/* Backdrop — click outside closes chat */}
      {open && (
        <div
          className={`fixed inset-0 z-40 ${expanded ? "bg-black/20 backdrop-blur-sm" : ""}`}
          onClick={() => { setOpen(false); setExpanded(false); }}
        />
      )}

      {/* FAB button — hidden when expanded */}
      {!expanded && (
        <button
          onClick={() => setOpen(!open)}
          className="fixed bottom-6 right-6 z-50 flex size-14 items-center justify-center rounded-full bg-[#3b6ef6] text-white shadow-lg transition-all hover:scale-105 hover:shadow-xl"
        >
          {open ? <X className="size-6" /> : <Sparkles className="size-6" />}
        </button>
      )}
    </>
  );
}
