import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { vi, describe, it, expect, beforeEach } from "vitest";
import { Sidebar } from "./Sidebar";

let mockPathname = "/inbox";
const replace = vi.fn();

vi.mock("next/navigation", () => ({
  usePathname: () => mockPathname,
  useRouter: () => ({ replace }),
}));

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const getUsage = vi.fn();
vi.mock("@/lib/api", () => ({
  api: {
    getUsage: (...args: unknown[]) => getUsage(...args),
  },
}));

type Usage = { plan: string; limit: number; month_leads: number; total_leads: number };
function usage(over: Partial<Usage> = {}): Usage {
  return { plan: "starter", limit: 100, month_leads: 25, total_leads: 50, ...over };
}

const NAV_LABELS = [
  "Входящие",
  "Напоминания",
  "Воронка",
  "Автоматизации",
  "Проспекты",
  "Секвенции",
  "Очередь отправки",
  "Аналитика",
  "Настройки",
  "Обучение",
];

describe("Sidebar", () => {
  beforeEach(() => {
    mockPathname = "/inbox";
    replace.mockReset();
    getUsage.mockReset().mockResolvedValue(usage());
    localStorage.clear();
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

  it("shows a loading placeholder before usage resolves", () => {
    getUsage.mockReturnValue(new Promise(() => {})); // never resolves
    render(<Sidebar />);
    expect(screen.getAllByText("Загрузка...").length).toBeGreaterThan(0);
    expect(screen.getAllByText("—").length).toBeGreaterThan(0);
  });

  it("renders the plan label and usage counter once usage loads", async () => {
    render(<Sidebar />);
    await waitFor(() => expect(screen.getAllByText("Starter").length).toBeGreaterThan(0));
    expect(screen.getAllByText("25 / 100 лидов в этом месяце").length).toBeGreaterThan(0);
  });

  it("offers an Upgrade link for non-pro plans", async () => {
    render(<Sidebar />);
    await waitFor(() => expect(screen.getAllByText("Upgrade").length).toBeGreaterThan(0));
  });

  it("hides the Upgrade link on the pro plan", async () => {
    getUsage.mockResolvedValue(usage({ plan: "pro" }));
    render(<Sidebar />);
    await waitFor(() => expect(screen.getAllByText("Pro").length).toBeGreaterThan(0));
    expect(screen.queryByText("Upgrade")).not.toBeInTheDocument();
  });

  it("falls back to the raw plan string when the label is unknown", async () => {
    getUsage.mockResolvedValue(usage({ plan: "enterprise" }));
    render(<Sidebar />);
    await waitFor(() => expect(screen.getAllByText("enterprise").length).toBeGreaterThan(0));
  });

  it("clears tokens and redirects to login on logout", () => {
    localStorage.setItem("token", "t");
    localStorage.setItem("refresh_token", "r");
    render(<Sidebar />);
    fireEvent.click(screen.getAllByText("Выход")[0]);
    expect(localStorage.getItem("token")).toBeNull();
    expect(localStorage.getItem("refresh_token")).toBeNull();
    expect(replace).toHaveBeenCalledWith("/login");
  });

  it("opens the mobile drawer from the hamburger and closes it via the close button", () => {
    render(<Sidebar />);
    expect(screen.queryByLabelText("Close menu")).not.toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("Open menu"));
    expect(screen.getByLabelText("Close menu")).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("Close menu"));
    expect(screen.queryByLabelText("Close menu")).not.toBeInTheDocument();
  });

  it("closes the mobile drawer on backdrop click", () => {
    const { container } = render(<Sidebar />);
    fireEvent.click(screen.getByLabelText("Open menu"));
    const backdrop = container.querySelector(".bg-black\\/40")!;
    fireEvent.click(backdrop);
    expect(screen.queryByLabelText("Close menu")).not.toBeInTheDocument();
  });

  it("closes the mobile drawer when Escape is pressed but ignores other keys", () => {
    render(<Sidebar />);
    fireEvent.click(screen.getByLabelText("Open menu"));
    fireEvent.keyDown(document, { key: "Enter" });
    expect(screen.getByLabelText("Close menu")).toBeInTheDocument();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByLabelText("Close menu")).not.toBeInTheDocument();
  });
});

describe("Sidebar — usage bar thresholds", () => {
  beforeEach(() => {
    mockPathname = "/inbox";
    getUsage.mockReset();
  });

  function fillBarColor() {
    const track = document.querySelector(".bg-slate-200")!;
    return (track.firstChild as HTMLElement).className;
  }

  it("uses the normal colour below 80% usage", async () => {
    getUsage.mockResolvedValue(usage({ month_leads: 25, limit: 100 }));
    render(<Sidebar />);
    await waitFor(() => expect(screen.getAllByText("Starter").length).toBeGreaterThan(0));
    expect(fillBarColor()).toContain("bg-[#004ac6]");
  });

  it("uses the amber colour when near the limit", async () => {
    getUsage.mockResolvedValue(usage({ month_leads: 85, limit: 100 }));
    render(<Sidebar />);
    await waitFor(() => expect(screen.getAllByText("Starter").length).toBeGreaterThan(0));
    expect(fillBarColor()).toContain("bg-amber-500");
  });

  it("uses the red colour and caps the width at 100% when over the limit", async () => {
    getUsage.mockResolvedValue(usage({ month_leads: 150, limit: 100 }));
    render(<Sidebar />);
    await waitFor(() => expect(screen.getAllByText("Starter").length).toBeGreaterThan(0));
    expect(fillBarColor()).toContain("bg-red-500");
    const fill = document.querySelector(".bg-red-500") as HTMLElement;
    expect(fill.style.width).toBe("100%");
  });
});
