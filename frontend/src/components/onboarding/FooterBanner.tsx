import { MessageCircle, Mail } from "lucide-react";

export function FooterBanner() {
  return (
    <section className="overflow-hidden rounded-2xl bg-gradient-to-br from-[#004ac6] to-[#2563eb] p-8 shadow-xl shadow-[#004ac6]/15">
      <div className="flex flex-col items-center gap-6 text-center sm:flex-row sm:text-left">
        <div className="flex-1">
          <h3 className="mb-2 text-lg font-extrabold text-white">
            Нужна помощь с настройкой?
          </h3>
          <p className="text-sm text-white/70">
            Мы поможем подключить все каналы и запустить первую рассылку.
            Бесплатно для всех тарифов.
          </p>
        </div>
        <div className="flex shrink-0 gap-3">
          <a
            href="https://t.me/floq_support"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 rounded-xl bg-white/15 px-5 py-3 text-sm font-bold text-white backdrop-blur-sm transition-all hover:bg-white/25"
          >
            <MessageCircle className="size-4" />
            Telegram
          </a>
          <a
            href="mailto:support@floq.ai"
            className="flex items-center gap-2 rounded-xl bg-white px-5 py-3 text-sm font-bold text-[#004ac6] shadow-md transition-all hover:-translate-y-0.5 hover:shadow-lg"
          >
            <Mail className="size-4" />
            Email
          </a>
        </div>
      </div>
    </section>
  );
}
