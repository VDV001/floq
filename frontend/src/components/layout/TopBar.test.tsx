import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { TopBar } from "./TopBar";

let mockPathname = "/inbox";

vi.mock("next/navigation", () => ({
  usePathname: () => mockPathname,
}));

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

describe("TopBar", () => {
  it("renders the search box and all nav tabs", () => {
    mockPathname = "/inbox";
    render(<TopBar />);
    expect(
      screen.getByPlaceholderText("Поиск лидов, компаний, сообщений...")
    ).toBeInTheDocument();
    expect(screen.getByText("Входящие")).toBeInTheDocument();
    expect(screen.getByText("Воронка")).toBeInTheDocument();
    expect(screen.getByText("Аналитика")).toBeInTheDocument();
    expect(screen.getByText("Новый лид")).toBeInTheDocument();
  });

  it("marks the active tab matching the current pathname", () => {
    mockPathname = "/inbox";
    render(<TopBar />);
    const active = screen.getByText("Входящие");
    expect(active).toHaveClass("text-[#3b6ef6]");
    // Inactive tabs use the muted color.
    expect(screen.getByText("Воронка")).toHaveClass("text-[#6b7280]");
  });

  it("marks a different tab active when navigated to a nested route", () => {
    mockPathname = "/analytics/overview";
    render(<TopBar />);
    expect(screen.getByText("Аналитика")).toHaveClass("text-[#3b6ef6]");
    expect(screen.getByText("Входящие")).toHaveClass("text-[#6b7280]");
  });
});
