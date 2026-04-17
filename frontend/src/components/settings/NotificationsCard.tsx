import { Bell } from "lucide-react";
import { Switch } from "@/components/ui/switch";

interface NotificationsCardProps {
  notifyTg: boolean;
  setNotifyTg: (v: boolean) => void;
  notifyEmail: boolean;
  setNotifyEmail: (v: boolean) => void;
  telegramBotActive: boolean;
}

export function NotificationsCard({ notifyTg, setNotifyTg, notifyEmail, setNotifyEmail, telegramBotActive }: NotificationsCardProps) {
  return (
    <div className="rounded-xl bg-white p-8 shadow-sm ring-1 ring-[#c3c6d7]/10">
      <div className="mb-6 flex items-center gap-3">
        <Bell className="size-5 text-[#004ac6]" />
        <h3 className="text-lg font-bold text-[#0d1c2e]">Уведомления</h3>
      </div>
      <div className="space-y-6">
        <label className="group flex cursor-pointer items-center justify-between">
          <div>
            <span className="text-sm font-medium text-[#434655] transition-colors group-hover:text-[#0d1c2e]">
              В Telegram о новых лидах
            </span>
            <p className="text-xs text-[#434655]/60">
              {telegramBotActive
                ? "Бот подключен — уведомления будут приходить"
                : "Сначала подключите Telegram бота"}
            </p>
          </div>
          <Switch checked={notifyTg} onCheckedChange={setNotifyTg} disabled={!telegramBotActive} />
        </label>
        <label className="group flex cursor-pointer items-center justify-between">
          <div>
            <span className="text-sm font-medium text-[#434655] transition-colors group-hover:text-[#0d1c2e]">
              Еженедельный отчет по email
            </span>
            <p className="text-xs text-[#434655]/60">Каждый понедельник — сводка по лидам и воронке</p>
          </div>
          <Switch checked={notifyEmail} onCheckedChange={setNotifyEmail} />
        </label>
      </div>
      <p className="mt-6 text-[11px] text-[#434655]/50">Изменения применяются после нажатия «Сохранить»</p>
    </div>
  );
}
