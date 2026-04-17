import {
  Send,
  Sparkles,
  UserPlus,
  GitBranch,
  Rocket,
  Mail,
  Brain,
  Shield,
  Bell,
} from "lucide-react";
import type { UserSettings } from "@/lib/api";

export interface StepDef {
  id: string;
  icon: React.ElementType;
  title: string;
  description: string;
  href: string;
  btnLabel: string;
  check: (s: UserSettings, counts: Counts) => boolean;
}

export interface Counts {
  prospects: number;
  sequences: number;
  outbound: number;
}

export const STEPS: StepDef[] = [
  {
    id: "telegram",
    icon: Send,
    title: "Подключите Telegram бота",
    description:
      "Создайте бота через @BotFather, скопируйте токен и вставьте в настройках. Входящие сообщения автоматически станут лидами.",
    href: "/settings",
    btnLabel: "Настроить",
    check: (s) => s.telegram_bot_active,
  },
  {
    id: "ai",
    icon: Sparkles,
    title: "Настройте AI-провайдер",
    description:
      "Выберите Claude, OpenAI, Groq или Ollama. AI будет квалифицировать лидов, генерировать ответы и холодные сообщения.",
    href: "/settings",
    btnLabel: "Настроить",
    check: (s) => s.ai_active,
  },
  {
    id: "email-out",
    icon: Mail,
    title: "Настройте отправку писем",
    description:
      "Подключите SMTP (mail.ru, Яндекс, Gmail) или Resend API для отправки холодных писем из секвенций.",
    href: "/settings",
    btnLabel: "Настроить",
    check: (s) => s.smtp_active || s.resend_active,
  },
  {
    id: "email-in",
    icon: Mail,
    title: "Подключите приём почты (IMAP)",
    description:
      "Настройте IMAP для автоматического приёма входящих писем. Ответы от проспектов будут создавать лидов автоматически.",
    href: "/settings",
    btnLabel: "Настроить",
    check: (s) => s.imap_active,
  },
  {
    id: "prospects",
    icon: UserPlus,
    title: "Добавьте первых проспектов",
    description:
      "Импортируйте CSV-файл с контактами или добавьте вручную. Верификация email встроена — плохие адреса отсеются.",
    href: "/prospects",
    btnLabel: "Добавить",
    check: (_, c) => c.prospects > 0,
  },
  {
    id: "sequence",
    icon: GitBranch,
    title: "Создайте секвенцию",
    description:
      "Настройте цепочку касаний: Email на старте → Telegram через 3 дня → Прозвон через неделю. AI напишет текст под каждый канал.",
    href: "/sequences",
    btnLabel: "Создать",
    check: (_, c) => c.sequences > 0,
  },
  {
    id: "launch",
    icon: Rocket,
    title: "Запустите первую рассылку",
    description:
      "Выберите проспектов, запустите секвенцию. Сообщения попадут в очередь одобрения — вы контролируете каждое касание.",
    href: "/outbound",
    btnLabel: "Запустить",
    check: (_, c) => c.outbound > 0,
  },
];

export const TIPS = [
  {
    icon: Brain,
    title: "AI-квалификация",
    description:
      "Каждый входящий лид оценивается по потребности, бюджету и срокам. Скор от 0 до 100 помогает фокусироваться на горячих.",
    accent: "from-[#004ac6] to-[#2563eb]",
  },
  {
    icon: Shield,
    title: "Верификация контактов",
    description:
      "Встроенный SMTP probe, MX lookup и фильтр одноразовых доменов. Проверяйте email до отправки — без платных сервисов.",
    accent: "from-[#059669] to-[#10b981]",
  },
  {
    icon: Bell,
    title: "Автоматические фоллоуапы",
    description:
      "Если лид молчит 2+ дня — Floq напомнит в Telegram. Ни один контакт не потеряется в воронке.",
    accent: "from-[#d97706] to-[#f59e0b]",
  },
];
