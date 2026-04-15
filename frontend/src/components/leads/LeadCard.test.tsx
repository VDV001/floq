import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { LeadCard, type LeadCardProps } from "./LeadCard";

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: React.ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const defaultProps: LeadCardProps = {
  id: "lead-1",
  company: "Acme Corp",
  contact: "john@acme.com",
  channel: "email",
  preview: "Interested in your product...",
  timeAgo: "2 ч назад",
  status: "Новый",
};

describe("LeadCard", () => {
  it("renders name, company and status", () => {
    render(<LeadCard {...defaultProps} />);
    expect(screen.getByText("Acme Corp")).toBeInTheDocument();
    expect(screen.getByText("Новый")).toBeInTheDocument();
    expect(screen.getByText("Interested in your product...")).toBeInTheDocument();
    expect(screen.getByText("2 ч назад")).toBeInTheDocument();
  });

  it("renders contact info", () => {
    render(<LeadCard {...defaultProps} />);
    expect(screen.getByText(/john@acme\.com/)).toBeInTheDocument();
  });

  it("shows correct badge for 'Новый' status", () => {
    render(<LeadCard {...defaultProps} status="Новый" />);
    const badge = screen.getByText("Новый");
    expect(badge.className).toContain("bg-[#3b6ef6]/10");
  });

  it("shows correct badge for 'Квалифицирован' status", () => {
    render(<LeadCard {...defaultProps} status="Квалифицирован" />);
    const badge = screen.getByText("Квалифицирован");
    expect(badge.className).toContain("border");
  });

  it("shows correct badge for 'Нужен фоллоуап' status", () => {
    render(<LeadCard {...defaultProps} status="Нужен фоллоуап" />);
    const badge = screen.getByText("Нужен фоллоуап");
    expect(badge.className).toContain("bg-[#f59e0b]/10");
  });

  it("links to the correct inbox page", () => {
    render(<LeadCard {...defaultProps} />);
    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("href", "/inbox/lead-1");
  });

  it("shows telegram channel correctly", () => {
    render(<LeadCard {...defaultProps} channel="telegram" />);
    expect(screen.getByText(/через Telegram/)).toBeInTheDocument();
  });
});
