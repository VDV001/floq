import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi, describe, it, expect, beforeEach } from "vitest";
import { FloatingActionButton } from "./FloatingActionButton";

let mockPathname = "/leads";

vi.mock("next/navigation", () => ({
  usePathname: () => mockPathname,
}));

// react-markdown is ESM-heavy; render its children verbatim so assistant text
// is assertable.
vi.mock("react-markdown", () => ({
  default: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

const chatWithAI = vi.fn();
const getUsage = vi.fn();

vi.mock("@/lib/api", () => ({
  api: {
    chatWithAI: (...args: unknown[]) => chatWithAI(...args),
    getUsage: (...args: unknown[]) => getUsage(...args),
  },
}));

beforeEach(() => {
  // jsdom doesn't implement scrollIntoView; the panel scrolls on every message.
  Element.prototype.scrollIntoView = vi.fn();
  mockPathname = "/leads";
  chatWithAI.mockReset().mockResolvedValue({ reply: "Привет от ассистента" });
  getUsage.mockReset().mockResolvedValue({
    plan: "starter",
    limit: 100,
    month_leads: 25,
    total_leads: 50,
  });
});

function openPanel() {
  // The FAB is the only button when closed.
  return userEvent.click(screen.getAllByRole("button")[0]);
}

describe("FloatingActionButton", () => {
  it("is closed initially and opens the chat panel on click", async () => {
    render(<FloatingActionButton />);
    expect(screen.queryByText("Чем могу помочь?")).not.toBeInTheDocument();

    await openPanel();
    expect(screen.getByText("Чем могу помочь?")).toBeInTheDocument();
    expect(screen.getByText("Floq AI")).toBeInTheDocument();
  });

  it("sends a message and renders the user text + assistant reply, passing the page context", async () => {
    render(<FloatingActionButton />);
    await openPanel();

    const input = screen.getByPlaceholderText("Спросите что-нибудь...");
    await userEvent.type(input, "Сколько лидов?{Enter}");

    await waitFor(() => expect(chatWithAI).toHaveBeenCalled());
    const [text, history, context] = chatWithAI.mock.calls[0];
    expect(text).toContain("Сколько лидов?");
    expect(Array.isArray(history)).toBe(true);
    expect(context).toBe("leads"); // derived from pathname "/leads"

    expect(await screen.findByText("Привет от ассистента")).toBeInTheDocument();
  });

  it("shows an error bubble when the AI call fails", async () => {
    chatWithAI.mockRejectedValue(new Error("сеть упала"));
    render(<FloatingActionButton />);
    await openPanel();

    const input = screen.getByPlaceholderText("Спросите что-нибудь...");
    await userEvent.type(input, "вопрос{Enter}");

    expect(await screen.findByText("Ошибка: сеть упала")).toBeInTheDocument();
  });

  it("disables submit while the input is empty", async () => {
    render(<FloatingActionButton />);
    await openPanel();

    // The send button is the only button inside the <form>; it's disabled with
    // empty input and enabled once the field has text.
    const sendBtn = document.querySelector("form button") as HTMLButtonElement;
    expect(sendBtn).toBeDisabled();

    await userEvent.type(screen.getByPlaceholderText("Спросите что-нибудь..."), "x");
    expect(sendBtn).toBeEnabled();
  });

  it("clears the conversation", async () => {
    render(<FloatingActionButton />);
    await openPanel();

    const input = screen.getByPlaceholderText("Спросите что-нибудь...");
    await userEvent.type(input, "первый{Enter}");
    expect(await screen.findByText("Привет от ассистента")).toBeInTheDocument();

    await userEvent.click(screen.getByTitle("Очистить чат"));
    expect(screen.queryByText("Привет от ассистента")).not.toBeInTheDocument();
    expect(screen.getByText("Чем могу помочь?")).toBeInTheDocument();
  });

  it("toggles the expanded view", async () => {
    render(<FloatingActionButton />);
    await openPanel();

    await userEvent.click(screen.getByTitle("Развернуть"));
    expect(screen.getByTitle("Компактный вид")).toBeInTheDocument();
    await userEvent.click(screen.getByTitle("Компактный вид"));
    expect(screen.getByTitle("Развернуть")).toBeInTheDocument();
  });

  it("fetches and shows the AI context panel, then toggles it off", async () => {
    render(<FloatingActionButton />);
    await openPanel();

    await userEvent.click(screen.getByTitle("Контекст AI"));
    await waitFor(() => expect(getUsage).toHaveBeenCalled());
    expect(await screen.findByText("Что видит AI")).toBeInTheDocument();
    expect(screen.getByText(/Лидов: 50/)).toBeInTheDocument();

    // Clicking again hides it.
    await userEvent.click(screen.getByTitle("Контекст AI"));
    await waitFor(() =>
      expect(screen.queryByText("Что видит AI")).not.toBeInTheDocument(),
    );
  });

  it("shows a fallback when the context fetch fails", async () => {
    getUsage.mockRejectedValue(new Error("nope"));
    render(<FloatingActionButton />);
    await openPanel();

    await userEvent.click(screen.getByTitle("Контекст AI"));
    expect(
      await screen.findByText("Не удалось загрузить контекст"),
    ).toBeInTheDocument();
  });

  it("alerts when voice input is unsupported by the browser", async () => {
    const alertSpy = vi.spyOn(window, "alert").mockImplementation(() => {});
    render(<FloatingActionButton />);
    await openPanel();

    await userEvent.click(screen.getByTitle("Голосовой ввод"));
    expect(alertSpy).toHaveBeenCalledWith("Браузер не поддерживает голосовой ввод");
    alertSpy.mockRestore();
  });

  it("defaults the derived context to 'dashboard' at the root path", async () => {
    mockPathname = "/";
    render(<FloatingActionButton />);
    await openPanel();

    const input = screen.getByPlaceholderText("Спросите что-нибудь...");
    await userEvent.type(input, "привет{Enter}");
    await waitFor(() => expect(chatWithAI).toHaveBeenCalled());
    expect(chatWithAI.mock.calls[0][2]).toBe("dashboard");
  });
});
