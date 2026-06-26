import { api } from "@/lib/api";
import type { NotificationInput } from "@/components/notifications/NotificationProvider";

type Notify = (n: NotificationInput) => void;
type NotifyError = (err: unknown, fallbackTitle?: string) => void;

// unarchiveLead drives the one user-facing "вернуть из архива" action shared by
// the archive list and the lead-detail page: it calls the API and emits the
// success/error toast with a single source of truth for the copy. Returns true
// only when the lead was actually unarchived, so each caller can apply its own
// post-success state update (drop the row / flip the detail affordance).
export async function unarchiveLead(
  id: string,
  notify: Notify,
  notifyError: NotifyError,
): Promise<boolean> {
  try {
    await api.unarchiveLead(id);
    notify({ type: "success", title: "Лид возвращён", message: "Он снова в ленте входящих." });
    return true;
  } catch (err) {
    notifyError(err, "Не удалось разархивировать лид");
    return false;
  }
}
