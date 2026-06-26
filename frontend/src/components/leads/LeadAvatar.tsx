import { Mail, Send } from "lucide-react";
import { cn } from "@/lib/utils";

// LeadAvatar is the channel-tinted icon tile shown at the start of every lead
// row. Extracted so the inbox LeadCard and the archive ArchivedLeadCard share
// one definition of the channel→icon→palette mapping instead of each carrying
// the same ternary + magic colour classes.
export function LeadAvatar({ channel }: { channel: "email" | "telegram" }) {
  return (
    <div
      className={cn(
        "flex size-12 shrink-0 items-center justify-center rounded-xl",
        channel === "email" ? "bg-[#dbe1ff]" : "bg-[#d5e0f8]"
      )}
    >
      {channel === "email" ? (
        <Mail className="size-5 text-[#004ac6]" />
      ) : (
        <Send className="size-5 text-[#229ED9]" />
      )}
    </div>
  );
}
