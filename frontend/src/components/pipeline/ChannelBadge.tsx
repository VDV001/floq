import { Mail, Send } from "lucide-react";

export function ChannelBadge({ channel }: { channel: "email" | "telegram" }) {
  if (channel === "telegram") {
    return (
      <div className="flex items-center gap-1.5">
        <div className="flex size-5 items-center justify-center rounded-md bg-sky-500/10"><Send className="size-3 text-sky-500" /></div>
        <span className="text-[11px] font-medium text-[#737686]">Telegram</span>
      </div>
    );
  }
  return (
    <div className="flex items-center gap-1.5">
      <div className="flex size-5 items-center justify-center rounded-md bg-red-500/10"><Mail className="size-3 text-red-500" /></div>
      <span className="text-[11px] font-medium text-[#737686]">Email</span>
    </div>
  );
}
