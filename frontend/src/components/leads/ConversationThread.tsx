"use client";

import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import type { Message } from "@/lib/api";

interface ConversationThreadProps {
  messages: Message[];
  leadName: string;
}

function formatTime(dateStr: string): string {
  const date = new Date(dateStr);
  return date.toLocaleTimeString("ru-RU", {
    hour: "numeric",
    minute: "2-digit",
    hour12: false,
  });
}

function formatDateSeparator(dateStr: string): string {
  const date = new Date(dateStr);
  const now = new Date();
  const yesterday = new Date(now);
  yesterday.setDate(yesterday.getDate() - 1);

  const options: Intl.DateTimeFormatOptions = {
    month: "long",
    day: "numeric",
  };
  const formatted = date.toLocaleDateString("ru-RU", options).toUpperCase();

  if (date.toDateString() === now.toDateString()) {
    return `СЕГОДНЯ, ${formatted}`;
  }
  if (date.toDateString() === yesterday.toDateString()) {
    return `ВЧЕРА, ${formatted}`;
  }
  return formatted;
}

function getDateKey(dateStr: string): string {
  return new Date(dateStr).toDateString();
}

function getInitials(name: string): string {
  return name
    .split(" ")
    .map((n) => n[0])
    .join("")
    .toUpperCase()
    .slice(0, 2);
}

export function ConversationThread({
  messages,
  leadName,
}: ConversationThreadProps) {
  // Group messages by date
  const grouped: { dateKey: string; dateSeparator: string; msgs: Message[] }[] =
    [];

  for (const msg of messages) {
    const key = getDateKey(msg.sent_at);
    const last = grouped[grouped.length - 1];
    if (last && last.dateKey === key) {
      last.msgs.push(msg);
    } else {
      grouped.push({
        dateKey: key,
        dateSeparator: formatDateSeparator(msg.sent_at),
        msgs: [msg],
      });
    }
  }

  return (
    <div className="space-y-6">
      {grouped.map((group) => (
        <div key={group.dateKey} className="space-y-4">
          {/* Date separator */}
          <div className="flex items-center gap-3">
            <div className="h-px flex-1 bg-gray-200" />
            <span className="text-xs font-semibold tracking-wider text-[#6b7280]">
              {group.dateSeparator}
            </span>
            <div className="h-px flex-1 bg-gray-200" />
          </div>

          {/* Messages */}
          {group.msgs.map((msg) => {
            const isInbound = msg.direction === "inbound";

            return (
              <div
                key={msg.id}
                className={`flex items-end gap-2.5 ${
                  isInbound ? "justify-start" : "justify-end"
                }`}
              >
                {isInbound && (
                  <Avatar size="sm">
                    <AvatarFallback className="bg-gray-200 text-xs text-[#0d1c2e]">
                      {getInitials(leadName)}
                    </AvatarFallback>
                  </Avatar>
                )}

                <div
                  className={`max-w-[70%] space-y-1 ${
                    isInbound ? "items-start" : "items-end"
                  }`}
                >
                  <div
                    className={`rounded-2xl px-4 py-2.5 text-sm leading-relaxed ${
                      isInbound
                        ? "rounded-bl-sm bg-[#f3f4f6] text-[#0d1c2e]"
                        : "rounded-br-sm bg-[#3b6ef6] text-white"
                    }`}
                  >
                    {msg.body}
                  </div>
                  <p
                    className={`text-[11px] text-[#6b7280] ${
                      isInbound ? "text-left" : "text-right"
                    }`}
                  >
                    {formatTime(msg.sent_at)}
                  </p>
                </div>

                {!isInbound && (
                  <Avatar size="sm">
                    <AvatarFallback className="bg-[#3b6ef6] text-xs text-white">
                      Flo
                    </AvatarFallback>
                  </Avatar>
                )}
              </div>
            );
          })}
        </div>
      ))}
    </div>
  );
}
