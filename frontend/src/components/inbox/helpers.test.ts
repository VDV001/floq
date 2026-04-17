import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { formatTime, formatDateLabel, groupMessagesByDate } from "./helpers";
import type { Message } from "@/lib/api";

describe("formatTime", () => {
  it("formats time in HH:MM format for ru-RU", () => {
    const result = formatTime("2026-04-17T14:30:00Z");
    // The exact output depends on timezone, but it should be a valid time string
    expect(result).toMatch(/^\d{2}:\d{2}$/);
  });

  it("returns consistent format for different times", () => {
    const morning = formatTime("2026-04-17T06:05:00Z");
    const evening = formatTime("2026-04-17T23:59:00Z");
    expect(morning).toMatch(/^\d{2}:\d{2}$/);
    expect(evening).toMatch(/^\d{2}:\d{2}$/);
  });

  it("handles midnight", () => {
    const result = formatTime("2026-04-17T00:00:00Z");
    expect(result).toMatch(/^\d{2}:\d{2}$/);
  });
});

describe("formatDateLabel", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-04-17T12:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns 'Сегодня' for today's date", () => {
    expect(formatDateLabel("2026-04-17T08:00:00Z")).toBe("Сегодня");
  });

  it("returns 'Вчера' for yesterday's date", () => {
    expect(formatDateLabel("2026-04-16T15:00:00Z")).toBe("Вчера");
  });

  it("returns formatted date for older dates", () => {
    const result = formatDateLabel("2026-04-10T10:00:00Z");
    // Should be a localized date like "10 апреля"
    expect(result).toBeTruthy();
    expect(result).not.toBe("Сегодня");
    expect(result).not.toBe("Вчера");
  });

  it("returns formatted date for a date two days ago", () => {
    const result = formatDateLabel("2026-04-15T10:00:00Z");
    expect(result).not.toBe("Сегодня");
    expect(result).not.toBe("Вчера");
  });

  it("returns formatted date for a date from different month", () => {
    const result = formatDateLabel("2026-03-01T10:00:00Z");
    expect(result).toBeTruthy();
    expect(result).not.toBe("Сегодня");
    expect(result).not.toBe("Вчера");
  });
});

describe("groupMessagesByDate", () => {
  const makeMessage = (id: string, sentAt: string): Message => ({
    id,
    lead_id: "lead-1",
    direction: "inbound",
    body: `Message ${id}`,
    sent_at: sentAt,
  });

  it("groups messages by date key (toDateString)", () => {
    const messages: Message[] = [
      makeMessage("1", "2026-04-17T10:00:00Z"),
      makeMessage("2", "2026-04-17T14:00:00Z"),
      makeMessage("3", "2026-04-16T09:00:00Z"),
    ];

    const groups = groupMessagesByDate(messages);
    // Messages on the same date should be in the same group
    const keys = Array.from(groups.keys());
    expect(keys.length).toBe(2);

    // Find the group with 2 messages
    const todayKey = new Date("2026-04-17T10:00:00Z").toDateString();
    const yesterdayKey = new Date("2026-04-16T09:00:00Z").toDateString();

    expect(groups.get(todayKey)?.length).toBe(2);
    expect(groups.get(yesterdayKey)?.length).toBe(1);
  });

  it("returns empty map for empty messages array", () => {
    const groups = groupMessagesByDate([]);
    expect(groups.size).toBe(0);
  });

  it("puts all same-date messages into one group", () => {
    // Use times well within the same day to avoid timezone boundary issues
    const messages: Message[] = [
      makeMessage("1", "2026-04-17T10:00:00Z"),
      makeMessage("2", "2026-04-17T12:00:00Z"),
      makeMessage("3", "2026-04-17T14:00:00Z"),
    ];

    const groups = groupMessagesByDate(messages);
    expect(groups.size).toBe(1);
    const values = Array.from(groups.values());
    expect(values[0].length).toBe(3);
  });

  it("preserves message order within groups", () => {
    const messages: Message[] = [
      makeMessage("first", "2026-04-17T08:00:00Z"),
      makeMessage("second", "2026-04-17T12:00:00Z"),
      makeMessage("third", "2026-04-17T18:00:00Z"),
    ];

    const groups = groupMessagesByDate(messages);
    const dateKey = new Date("2026-04-17T08:00:00Z").toDateString();
    const group = groups.get(dateKey);
    expect(group?.[0].id).toBe("first");
    expect(group?.[1].id).toBe("second");
    expect(group?.[2].id).toBe("third");
  });

  it("handles messages spanning multiple days", () => {
    const messages: Message[] = [
      makeMessage("1", "2026-04-15T10:00:00Z"),
      makeMessage("2", "2026-04-16T10:00:00Z"),
      makeMessage("3", "2026-04-17T10:00:00Z"),
    ];

    const groups = groupMessagesByDate(messages);
    expect(groups.size).toBe(3);
  });

  it("single message creates a single group", () => {
    const messages: Message[] = [
      makeMessage("only", "2026-04-17T10:00:00Z"),
    ];

    const groups = groupMessagesByDate(messages);
    expect(groups.size).toBe(1);
    const values = Array.from(groups.values());
    expect(values[0].length).toBe(1);
    expect(values[0][0].id).toBe("only");
  });
});
