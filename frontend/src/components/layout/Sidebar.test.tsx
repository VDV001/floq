import { render, screen } from "@testing-library/react";
import { vi, describe, it, expect, beforeEach } from "vitest";
import { Sidebar } from "./Sidebar";

let mockPathname = "/inbox";

vi.mock("next/navigation", () => ({
  usePathname: () => mockPathname,
  useRouter: () => ({ replace: vi.fn() }),
}));

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock("@/lib/api", () => ({
  api: {
    getUsage: vi.fn().mockResolvedValue({
      plan: "starter",
      limit: 100,
      month_leads: 25,
      total_leads: 50,
    }),
  },
}));

const NAV_LABELS = [
  "Входящие",
  "Лиды",
  "Воронка",
  "Автоматизации",
  "Проспекты",
  "Секвенции",
  "Очередь отправки",
  "Настройки",
  "Обучение",
];

describe("Sidebar", () => {
  beforeEach(() => {
    mockPathname = "/inbox";
  });

  it("renders all navigation links", () => {
    render(<Sidebar />);
    for (const label of NAV_LABELS) {
      expect(screen.getAllByText(label).length).toBeGreaterThanOrEqual(1);
    }
  });

  it("highlights active link", () => {
    mockPathname = "/pipeline";
    render(<Sidebar />);
    // The sidebar renders content twice (mobile + desktop), find the link elements
    const links = screen.getAllByText("Воронка");
    // At least one should have the active class
    const hasActive = links.some((el) => {
      const anchor = el.closest("a");
      return anchor?.className.includes("font-bold");
    });
    expect(hasActive).toBe(true);
  });

  it("does not highlight non-active links", () => {
    mockPathname = "/pipeline";
    render(<Sidebar />);
    const links = screen.getAllByText("Входящие");
    const hasActive = links.some((el) => {
      const anchor = el.closest("a");
      return anchor?.className.includes("font-bold");
    });
    expect(hasActive).toBe(false);
  });
});
