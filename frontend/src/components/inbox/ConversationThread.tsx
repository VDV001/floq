import { User } from "lucide-react";
import type { Message } from "@/lib/api";
import { formatTime, formatDateLabel, groupMessagesByDate } from "./helpers";

interface ConversationThreadProps {
  messages: Message[];
  initials: string;
}

export function ConversationThread({ messages, initials }: ConversationThreadProps) {
  if (messages.length === 0) {
    return <p className="text-center text-sm text-[#434655]">Нет сообщений</p>;
  }

  const messageGroups = groupMessagesByDate(messages);

  return (
    <>
      {Array.from(messageGroups.entries()).map(([dateKey, groupMsgs]) => (
        <div key={dateKey}>
          <div className="mb-8 flex items-center gap-4">
            <div className="h-px flex-1 bg-[#c3c6d7]/20" />
            <span className="text-[0.7rem] font-bold uppercase tracking-widest text-[#737686]">{formatDateLabel(groupMsgs[0].sent_at)}</span>
            <div className="h-px flex-1 bg-[#c3c6d7]/20" />
          </div>
          <div className="space-y-8">
            {groupMsgs.map((msg) =>
              msg.direction === "inbound" ? (
                <div key={msg.id} className="flex items-start gap-4">
                  <div className="flex size-8 shrink-0 items-center justify-center rounded-full bg-[#dbe1ff] text-xs font-bold text-[#004ac6]">{initials}</div>
                  <div className="max-w-[80%] rounded-2xl rounded-tl-none bg-[#dce9ff] p-4">
                    <p className="text-sm leading-relaxed text-[#0d1c2e]">{msg.body}</p>
                    <span className="mt-2 block text-[0.6rem] text-[#434655]">{formatTime(msg.sent_at)}</span>
                  </div>
                </div>
              ) : (
                <div key={msg.id} className="flex flex-row-reverse items-start gap-4">
                  <div className="flex size-8 shrink-0 items-center justify-center rounded-full bg-[#004ac6] text-white"><User className="size-4" /></div>
                  <div className="max-w-[80%] rounded-2xl rounded-tr-none bg-[#004ac6] p-4 text-white shadow-sm">
                    <p className="text-sm leading-relaxed">{msg.body}</p>
                    <span className="mt-2 block text-[0.6rem] text-[#dbe1ff] opacity-80">{formatTime(msg.sent_at)}</span>
                  </div>
                </div>
              )
            )}
          </div>
        </div>
      ))}
    </>
  );
}
