"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  Inbox,
  Users,
  GitBranch,
  Zap,
  Settings,
  Sparkles,
  LogOut,
  HelpCircle,
  UserPlus,
  Send,
} from "lucide-react";
import { cn } from "@/lib/utils";

const NAV_ITEMS = [
  { label: "Входящие", href: "/inbox", icon: Inbox },
  { label: "Лиды", href: "/alerts", icon: Users },
  { label: "Воронка", href: "/pipeline", icon: GitBranch },
  { label: "Автоматизации", href: "/automations", icon: Zap },
  { label: "Проспекты", href: "/prospects", icon: UserPlus },
  { label: "Секвенции", href: "/sequences", icon: GitBranch },
  { label: "Очередь отправки", href: "/outbound", icon: Send },
  { label: "Настройки", href: "/settings", icon: Settings },
];

export function Sidebar() {
  const pathname = usePathname();
  const router = useRouter();

  const handleLogout = () => {
    localStorage.removeItem("token");
    localStorage.removeItem("refresh_token");
    router.replace("/login");
  };

  return (
    <aside className="flex w-64 shrink-0 flex-col bg-[#eff4ff]">
      {/* Logo */}
      <div className="px-6 pt-8 pb-6">
        <div className="flex items-center gap-3">
          <Sparkles className="size-8 text-[#3b6ef6]" />
          <div>
            <h1 className="text-2xl font-bold tracking-tight text-[#0d1c2e]">
              Floq
            </h1>
            <p className="text-[10px] font-bold uppercase tracking-widest text-[#434655]/60">
              AI Sales Assistant
            </p>
          </div>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 space-y-1 px-4">
        {NAV_ITEMS.map((item) => {
          const isActive = pathname.startsWith(item.href);
          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                "flex items-center gap-3 rounded-lg px-4 py-3 text-sm font-medium transition-colors duration-200",
                isActive
                  ? "bg-white/50 font-bold text-[#2563eb]"
                  : "text-[#434655] hover:bg-[#dce9ff]"
              )}
            >
              <item.icon className="size-5" />
              {item.label}
            </Link>
          );
        })}
      </nav>

      {/* Bottom section */}
      <div className="mt-auto space-y-4 px-4 pb-8">
        {/* Pro upsell */}
        <div className="rounded-xl bg-[#dbe1ff]/30 p-4">
          <p className="mb-2 text-xs font-bold text-[#004ac6]">
            Upgrade to Pro
          </p>
          <div className="mb-2 h-1.5 w-full overflow-hidden rounded-full bg-slate-200">
            <div className="h-full w-3/4 bg-[#004ac6]" />
          </div>
          <p className="text-[10px] text-slate-500">750/1000 лимитов</p>
        </div>

        {/* Links */}
        <div className="space-y-1">
          <button className="flex w-full items-center gap-3 rounded-lg px-4 py-2 text-sm font-medium text-[#434655] transition-colors hover:text-[#0d1c2e]">
            <HelpCircle className="size-5" />
            Поддержка
          </button>
          <button
            onClick={handleLogout}
            className="flex w-full items-center gap-3 rounded-lg px-4 py-2 text-sm font-medium text-[#434655] transition-colors hover:text-[#ba1a1a]"
          >
            <LogOut className="size-5" />
            Выход
          </button>
        </div>
      </div>
    </aside>
  );
}
