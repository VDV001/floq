import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { ConversationThread } from "./ConversationThread";
import type { Message } from "@/lib/api";

function msg(over: Partial<Message> = {}): Message {
  return {
    id: "m1",
    lead_id: "lead-1",
    body: "Привет",
    direction: "inbound",
    sent_at: new Date().toISOString(),
    ...over,
  };
}

function isoDaysAgo(days: number): string {
  const d = new Date();
  d.setDate(d.getDate() - days);
  d.setHours(10, 30, 0, 0);
  return d.toISOString();
}

describe("inbox/ConversationThread", () => {
  it("renders an empty-state message for no messages", () => {
    render(<ConversationThread messages={[]} initials="ИП" />);
    expect(screen.getByText("Нет сообщений")).toBeInTheDocument();
  });

  it("renders inbound bubbles with the lead initials and outbound bubbles without them", () => {
    render(
      <ConversationThread
        messages={[
          msg({ id: "a", body: "Входящее", direction: "inbound", sent_at: isoDaysAgo(0) }),
          msg({ id: "b", body: "Исходящее", direction: "outbound", sent_at: isoDaysAgo(0) }),
        ]}
        initials="ИП"
      />,
    );
    expect(screen.getByText("Входящее")).toBeInTheDocument();
    expect(screen.getByText("Исходящее")).toBeInTheDocument();
    // Initials only render for the inbound avatar.
    expect(screen.getAllByText("ИП")).toHaveLength(1);
  });

  it("groups same-day messages under one separator and opens a new group for a different day", () => {
    render(
      <ConversationThread
        messages={[
          msg({ id: "a", body: "сегодня-1", sent_at: isoDaysAgo(0) }),
          msg({ id: "b", body: "сегодня-2", sent_at: isoDaysAgo(0) }),
          msg({ id: "c", body: "старое", sent_at: "2020-03-15T10:00:00Z" }),
        ]}
        initials="ИП"
      />,
    );
    expect(screen.getByText("сегодня-1")).toBeInTheDocument();
    expect(screen.getByText("старое")).toBeInTheDocument();
    expect(screen.getAllByText("Сегодня")).toHaveLength(1);
  });

  it("labels the today and yesterday separators", () => {
    render(<ConversationThread messages={[msg({ sent_at: isoDaysAgo(0) })]} initials="A" />);
    expect(screen.getByText("Сегодня")).toBeInTheDocument();

    render(<ConversationThread messages={[msg({ sent_at: isoDaysAgo(1) })]} initials="A" />);
    expect(screen.getByText("Вчера")).toBeInTheDocument();
  });

  it("uses a plain date label for older messages", () => {
    render(
      <ConversationThread messages={[msg({ sent_at: "2020-03-15T10:00:00Z" })]} initials="A" />,
    );
    expect(screen.queryByText("Сегодня")).not.toBeInTheDocument();
    expect(screen.queryByText("Вчера")).not.toBeInTheDocument();
    expect(screen.getByText(/марта/)).toBeInTheDocument();
  });
});
