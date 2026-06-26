import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { ConversationThread } from "./ConversationThread";
import type { Message } from "@/lib/api";

function msg(over: Partial<Message> = {}): Message {
  return {
    id: "m1",
    body: "Привет",
    direction: "inbound",
    sent_at: new Date().toISOString(),
    ...over,
  } as Message;
}

function isoDaysAgo(days: number): string {
  const d = new Date();
  d.setDate(d.getDate() - days);
  d.setHours(10, 30, 0, 0);
  return d.toISOString();
}

describe("ConversationThread", () => {
  it("renders inbound and outbound bubbles with the right avatars and bodies", () => {
    render(
      <ConversationThread
        leadName="Иван Петров"
        messages={[
          msg({ id: "a", body: "Входящее", direction: "inbound" }),
          msg({ id: "b", body: "Исходящее", direction: "outbound" }),
        ]}
      />,
    );
    expect(screen.getByText("Входящее")).toBeInTheDocument();
    expect(screen.getByText("Исходящее")).toBeInTheDocument();
    expect(screen.getByText("ИП")).toBeInTheDocument(); // inbound → lead initials
    expect(screen.getByText("Flo")).toBeInTheDocument(); // outbound → bot avatar
  });

  it("groups same-day messages and opens a new group on a different day", () => {
    render(
      <ConversationThread
        leadName="Alice"
        messages={[
          msg({ id: "a", body: "сегодня-1", sent_at: isoDaysAgo(0) }),
          msg({ id: "b", body: "сегодня-2", sent_at: isoDaysAgo(0) }),
          msg({ id: "c", body: "старое", sent_at: "2020-03-15T10:00:00Z" }),
        ]}
      />,
    );
    // Both same-day branch (push to last group) and new-day branch are taken.
    expect(screen.getByText("сегодня-1")).toBeInTheDocument();
    expect(screen.getByText("сегодня-2")).toBeInTheDocument();
    expect(screen.getByText("старое")).toBeInTheDocument();
    // Exactly one "СЕГОДНЯ" separator for the two same-day messages.
    expect(screen.getAllByText(/СЕГОДНЯ/).length).toBe(1);
  });

  it("labels the today separator", () => {
    render(<ConversationThread leadName="A" messages={[msg({ sent_at: isoDaysAgo(0) })]} />);
    expect(screen.getByText(/СЕГОДНЯ/)).toBeInTheDocument();
  });

  it("labels the yesterday separator", () => {
    render(<ConversationThread leadName="A" messages={[msg({ sent_at: isoDaysAgo(1) })]} />);
    expect(screen.getByText(/ВЧЕРА/)).toBeInTheDocument();
  });

  it("uses a plain date for older messages (no today/yesterday prefix)", () => {
    render(
      <ConversationThread
        leadName="A"
        messages={[msg({ sent_at: "2020-03-15T10:00:00Z" })]}
      />,
    );
    expect(screen.queryByText(/СЕГОДНЯ/)).not.toBeInTheDocument();
    expect(screen.queryByText(/ВЧЕРА/)).not.toBeInTheDocument();
  });

  it("renders nothing meaningful for an empty thread", () => {
    const { container } = render(<ConversationThread leadName="A" messages={[]} />);
    expect(container.querySelectorAll("p").length).toBe(0);
  });
});
