import { Lock, Camera } from "lucide-react";
import type { UserSettings } from "@/lib/api";

export function ProfileCard({ settings }: { settings: UserSettings | null }) {
  const initials = settings?.full_name
    ? settings.full_name.split(" ").map((w) => w[0]).join("").toUpperCase().slice(0, 2)
    : "??";

  return (
    <div className="rounded-xl bg-white p-8 shadow-sm ring-1 ring-[#c3c6d7]/10">
      <div className="flex flex-col items-center text-center">
        <div className="group relative mb-4 cursor-pointer">
          <div className="flex size-24 items-center justify-center rounded-full border-4 border-[#eff4ff] bg-[#dbe1ff] text-2xl font-bold text-[#004ac6] shadow-md">
            {initials}
          </div>
          <div className="absolute inset-0 flex items-center justify-center rounded-full bg-black/40 opacity-0 transition-opacity group-hover:opacity-100">
            <Camera className="size-5 text-white" />
          </div>
        </div>
        <h3 className="text-xl font-bold text-[#0d1c2e]">{settings?.full_name || "—"}</h3>
        <p className="mb-6 text-sm text-[#434655]">{settings?.email || "—"}</p>
        <button className="flex w-full items-center justify-center gap-2 rounded-lg bg-[#eff4ff] px-4 py-2.5 text-sm font-semibold text-[#434655] transition-colors hover:bg-[#dce9ff]">
          <Lock className="size-[18px]" />
          Сменить пароль
        </button>
      </div>
    </div>
  );
}
