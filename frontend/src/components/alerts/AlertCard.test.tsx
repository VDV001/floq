import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { AlertCard } from "./AlertCard";

// Mock the Button component to avoid base-ui dependency issues
vi.mock("@/components/ui/button", () => ({
  Button: ({ children, ...props }: { children: React.ReactNode; [key: string]: unknown }) => (
    <button {...props}>{children}</button>
  ),
}));

const defaultProps = {
  name: "Иван Петров",
  company: "Tech Solutions",
  title: "CTO",
  initials: "ИП",
  lastContact: "3 дня назад",
  action: "Отправить фоллоуап",
  avatarColor: "#3b6ef6",
};

describe("AlertCard", () => {
  it("renders name and company", () => {
    render(<AlertCard {...defaultProps} />);
    expect(screen.getByText("Иван Петров")).toBeInTheDocument();
    expect(screen.getByText(/Tech Solutions/)).toBeInTheDocument();
  });

  it("renders title alongside company", () => {
    render(<AlertCard {...defaultProps} />);
    expect(screen.getByText(/CTO/)).toBeInTheDocument();
  });

  it("renders initials in avatar", () => {
    render(<AlertCard {...defaultProps} />);
    expect(screen.getByText("ИП")).toBeInTheDocument();
  });

  it("renders last contact info", () => {
    render(<AlertCard {...defaultProps} />);
    expect(screen.getByText("3 дня назад")).toBeInTheDocument();
  });

  it("renders action text", () => {
    render(<AlertCard {...defaultProps} />);
    expect(screen.getByText(/Отправить фоллоуап/)).toBeInTheDocument();
  });

  it("applies avatar background color", () => {
    render(<AlertCard {...defaultProps} />);
    const avatar = screen.getByText("ИП");
    expect(avatar).toHaveStyle({ backgroundColor: "#3b6ef6" });
  });
});
