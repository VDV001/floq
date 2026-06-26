import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi, describe, it, expect, beforeEach } from "vitest";
import { MessageCard } from "./MessageCard";
import type { UIMessage } from "./constants";

const editMessage = vi.fn();
vi.mock("@/lib/api", () => ({
  api: { editMessage: (...a: unknown[]) => editMessage(...a) },
}));

function msg(over: Partial<UIMessage> = {}): UIMessage {
  return {
    id: "m1",
    name: "Alice",
    role: "CEO",
    initials: "AL",
    avatarBg: "bg-blue-100",
    step: "STEP 1",
    sequence: "Cold",
    channel: "email",
    body: "Привет",
    scheduledAt: "завтра",
    status: "approved",
    ...over,
  } as UIMessage;
}

beforeEach(() => editMessage.mockReset().mockResolvedValue(undefined));

const cbs = () => ({ onApprove: vi.fn(), onReject: vi.fn(), onEdited: vi.fn() });

describe("MessageCard", () => {
  it("renders identity, body and channel label", () => {
    render(<MessageCard msg={msg()} isQueue {...cbs()} />);
    expect(screen.getByText("Alice")).toBeInTheDocument();
    expect(screen.getByText("CEO")).toBeInTheDocument();
    expect(screen.getByText(/Привет/)).toBeInTheDocument();
    expect(screen.getByText("Email")).toBeInTheDocument();
  });

  it.each([
    ["telegram", "Telegram"],
    ["call", "Звонок"],
  ])("labels the %s channel", (channel, label) => {
    render(<MessageCard msg={msg({ channel: channel as UIMessage["channel"] })} isQueue {...cbs()} />);
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it("fires approve and reject in queue mode", async () => {
    const c = cbs();
    render(<MessageCard msg={msg({ id: "x" })} isQueue {...c} />);
    await userEvent.click(screen.getByText("Подтвердить"));
    expect(c.onApprove).toHaveBeenCalledWith("x");

    // The reject button is the icon-only X (last button).
    const buttons = screen.getAllByRole("button");
    await userEvent.click(buttons[buttons.length - 1]);
    expect(c.onReject).toHaveBeenCalledWith("x");
  });

  it("edits and saves a message through the API", async () => {
    const c = cbs();
    render(<MessageCard msg={msg({ id: "x", body: "старое" })} isQueue {...c} />);

    // Pencil edit button is the middle action button.
    const buttons = screen.getAllByRole("button");
    await userEvent.click(buttons[1]);
    const textarea = screen.getByRole("textbox");
    expect(textarea).toHaveValue("старое");
    await userEvent.clear(textarea);
    await userEvent.type(textarea, "новое");
    await userEvent.click(screen.getByText("Сохранить"));

    await waitFor(() => expect(editMessage).toHaveBeenCalledWith("x", "новое"));
    expect(c.onEdited).toHaveBeenCalledWith("x", "новое");
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  });

  it("cancels an edit without calling the API", async () => {
    const c = cbs();
    render(<MessageCard msg={msg()} isQueue {...c} />);
    await userEvent.click(screen.getAllByRole("button")[1]);
    await userEvent.click(screen.getByText("Отмена"));
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
    expect(editMessage).not.toHaveBeenCalled();
  });

  it.each([
    ["sent", "Отправлено"],
    ["rejected", "Отклонено"],
    ["approved", "Одобрено"],
  ])("shows the %s status badge (not action buttons) outside the queue", (status, label) => {
    render(<MessageCard msg={msg({ status: status as UIMessage["status"] })} isQueue={false} {...cbs()} />);
    expect(screen.getByText(label)).toBeInTheDocument();
    expect(screen.queryByText("Подтвердить")).not.toBeInTheDocument();
  });
});
