import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { FeaturedCard } from "./FeaturedCard";
import type { Lead } from "@/lib/api";

function lead(over: Partial<Lead> = {}): Lead {
  return {
    id: "l-1",
    user_id: "u-1",
    channel: "telegram",
    contact_name: "Мария Сидорова",
    company: "Globex",
    first_message: "Нужна интеграция",
    status: "followup",
    created_at: "2026-06-20T10:00:00Z",
    updated_at: "2026-06-24T10:00:00Z",
    ...over,
  };
}

describe("FeaturedCard", () => {
  it("renders contact, initials and telegram channel", () => {
    render(<FeaturedCard featured={lead()} />);
    expect(screen.getByText("Мария Сидорова")).toBeInTheDocument();
    expect(screen.getByText("МС")).toBeInTheDocument();
    expect(screen.getByText(/Globex/)).toBeInTheDocument();
    expect(screen.getByText(/Telegram/)).toBeInTheDocument();
  });

  it("shows Email label and dash company fallback", () => {
    render(<FeaturedCard featured={lead({ channel: "email", company: "" })} />);
    expect(screen.getByText(/Email/)).toBeInTheDocument();
    expect(screen.getByText(/—/)).toBeInTheDocument();
  });

  it("truncates a long last message with ellipsis", () => {
    const long = "b".repeat(200);
    render(<FeaturedCard featured={lead({ first_message: long })} />);
    expect(
      screen.getByText(new RegExp(`Последнее сообщение: "${"b".repeat(120)}\\.\\.\\."`))
    ).toBeInTheDocument();
  });

  it("shows short last message without ellipsis", () => {
    render(<FeaturedCard featured={lead({ first_message: "Привет" })} />);
    expect(screen.getByText(/Последнее сообщение: "Привет"/)).toBeInTheDocument();
  });

  it("shows fallback suggestion when there is no first message", () => {
    render(<FeaturedCard featured={lead({ first_message: "" })} />);
    expect(
      screen.getByText(/Floq рекомендует связаться с лидом/)
    ).toBeInTheDocument();
  });

  it("renders silent-days badge of at least 1 day", () => {
    render(<FeaturedCard featured={lead()} />);
    expect(screen.getByText(/Молчит \d+ д/)).toBeInTheDocument();
  });
});
