import type { ReactNode } from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import type { UserSettings, Prospect, Sequence, OutboundMessage } from "@/lib/api";
import OnboardingPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), back: vi.fn() }),
  usePathname: () => "/onboarding",
}));

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const mockSettings = {
  telegram_bot_active: false,
  ai_active: true,
  smtp_active: false,
  resend_active: false,
  imap_active: false,
};

vi.mock("@/lib/api", () => ({
  api: {
    getSettings: vi.fn(),
    getProspects: vi.fn(),
    getSequences: vi.fn(),
    getOutboundQueue: vi.fn(),
  },
}));

import { api } from "@/lib/api";

describe("OnboardingPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getSettings).mockResolvedValue(mockSettings as UserSettings);
    vi.mocked(api.getProspects).mockResolvedValue([]);
    vi.mocked(api.getSequences).mockResolvedValue([]);
    vi.mocked(api.getOutboundQueue).mockResolvedValue([]);
  });

  it("renders onboarding header", async () => {
    render(<OnboardingPage />);

    await waitFor(() => {
      expect(screen.getByText("Добро пожаловать в Floq")).toBeInTheDocument();
    });
  });

  it("renders progress indicator", async () => {
    render(<OnboardingPage />);

    await waitFor(() => {
      expect(screen.getByText(/1 \/ 7/)).toBeInTheDocument();
    });
  });

  it("renders step titles", async () => {
    render(<OnboardingPage />);

    await waitFor(() => {
      expect(screen.getByText("Подключите Telegram бота")).toBeInTheDocument();
      expect(screen.getByText("Настройте AI-провайдер")).toBeInTheDocument();
      expect(screen.getByText("Создайте секвенцию")).toBeInTheDocument();
    });
  });

  it("marks completed steps", async () => {
    render(<OnboardingPage />);

    await waitFor(() => {
      // AI is active, so it should be marked as done
      expect(screen.getByText("Готово")).toBeInTheDocument();
    });
  });

  it("renders tips section", async () => {
    render(<OnboardingPage />);

    await waitFor(() => {
      expect(screen.getByText("Полезные возможности")).toBeInTheDocument();
      expect(screen.getByText("AI-квалификация")).toBeInTheDocument();
    });
  });

  it("renders the how-it-works overview with both engines and the pipeline", async () => {
    render(<OnboardingPage />);

    await waitFor(() => {
      expect(screen.getByText("Как работает Floq")).toBeInTheDocument();
      expect(screen.getByText("Входящий поток")).toBeInTheDocument();
      expect(screen.getByText("Исходящий поток")).toBeInTheDocument();
      expect(screen.getByText("Путь лида по воронке")).toBeInTheDocument();
      expect(screen.getByText("Квалифицирован")).toBeInTheDocument();
    });
  });

  it("renders the sections map and glossary", async () => {
    render(<OnboardingPage />);

    await waitFor(() => {
      expect(screen.getByText("Разделы системы")).toBeInTheDocument();
      expect(screen.getByText("Очередь отправки")).toBeInTheDocument();
      expect(screen.getByText("Словарь терминов")).toBeInTheDocument();
      // The lead-vs-prospect distinction is the most common confusion.
      expect(screen.getByText("Проспект")).toBeInTheDocument();
    });
  });

  it("renders an expandable step-by-step guide for each step", async () => {
    render(<OnboardingPage />);

    await waitFor(() => {
      // One "Подробная инструкция" disclosure per step (7).
      expect(screen.getAllByText("Подробная инструкция")).toHaveLength(7);
      // Concrete numbered walkthrough copy is present (the /newbot command only
      // appears in the detailed guide, not in the short step description).
      expect(screen.getByText(/\/newbot/)).toBeInTheDocument();
      // Every step's guide carries a "Зачем" rationale and a "Что дальше" result.
      expect(screen.getAllByText(/Зачем:/)).toHaveLength(7);
    });
  });

  it("shows all-done state when all steps complete", async () => {
    vi.mocked(api.getSettings).mockResolvedValue({
      ...mockSettings,
      telegram_bot_active: true,
      ai_active: true,
      smtp_active: true,
      imap_active: true,
    } as UserSettings);
    vi.mocked(api.getProspects).mockResolvedValue([{ id: "p1" }] as Prospect[]);
    vi.mocked(api.getSequences).mockResolvedValue([{ id: "s1" }] as Sequence[]);
    vi.mocked(api.getOutboundQueue).mockResolvedValue([{ id: "o1" }] as OutboundMessage[]);

    render(<OnboardingPage />);

    await waitFor(() => {
      expect(screen.getByText("Всё готово!")).toBeInTheDocument();
    });
  });
});
