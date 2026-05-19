import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { LeadCard, type LeadCardProps } from "./LeadCard";
import { STATUS_STYLES } from "@/components/leads/constants";

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

  // Table-driven: every status in the inbox STATUS_STYLES map must render
  // with the exact same style class. This pins LeadCard as a drop-in for
  // the inline JSX previously in inbox/page.tsx — no visual regression
  // when swapping. Covers all 6 statuses (was 3 in earlier iteration).
  it.each([
    ["Новый"],
    ["Квалифицирован"],
    ["В диалоге"],
    ["Нужен фоллоуап"],
    ["Закрыт"],
    ["Выигран"],
  ] as const)("renders status badge with inbox style for '%s'", (status) => {
    render(<LeadCard {...defaultProps} status={status} />);
    const badge = screen.getByText(status);
    expect(badge.className).toContain(STATUS_STYLES[status]);
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

  it("shows no pending-reply badge when count is zero or undefined", () => {
    render(<LeadCard {...defaultProps} pendingRepliesCount={0} />);
    expect(screen.queryByLabelText(/ожидают подтверждения/i)).not.toBeInTheDocument();
  });

  it("shows pending-reply badge with count when greater than zero", () => {
    render(<LeadCard {...defaultProps} pendingRepliesCount={3} />);
    const badge = screen.getByLabelText(/ожидают подтверждения/i);
    expect(badge).toBeInTheDocument();
    expect(badge.textContent).toContain("3");
  });

  it("renders sourceName chip when provided", () => {
    render(<LeadCard {...defaultProps} sourceName="LinkedIn" />);
    expect(screen.getByText("LinkedIn")).toBeInTheDocument();
  });

  it("omits sourceName chip when not provided", () => {
    render(<LeadCard {...defaultProps} />);
    // No source = no chip rendered. We pick a label unlikely to collide
    // with any other text the card emits.
    expect(screen.queryByText("LinkedIn")).not.toBeInTheDocument();
  });

  it("renders suggestion-count badge when count > 0", () => {
    render(<LeadCard {...defaultProps} suggestionCount={2} />);
    const badge = screen.getByLabelText(/возможных совпадений/i);
    expect(badge).toBeInTheDocument();
    expect(badge.textContent).toContain("2");
  });

  it("omits suggestion-count badge when count is zero or undefined", () => {
    render(<LeadCard {...defaultProps} suggestionCount={0} />);
    expect(screen.queryByLabelText(/возможных совпадений/i)).not.toBeInTheDocument();
  });
});
