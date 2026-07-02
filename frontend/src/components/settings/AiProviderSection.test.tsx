import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { AiProviderSection } from "./AiProviderSection";

type Props = React.ComponentProps<typeof AiProviderSection>;

function setup(over: Partial<Props> = {}) {
  const props: Props = {
    aiProvider: "ollama",
    setAiProvider: vi.fn(),
    aiModel: "gemma3:4b",
    setAiModel: vi.fn(),
    aiApiKey: "",
    setAiApiKey: vi.fn(),
    maskedKey: "",
    showApiKey: false,
    setShowApiKey: vi.fn(),
    active: false,
    testing: false,
    testResult: null,
    setTestResult: vi.fn(),
    hasKey: false,
    providerDefaults: { claude: "claude-opus-4-8", openai: "gpt-4o" },
    onTest: vi.fn(),
    ...over,
  };
  render(<AiProviderSection {...props} />);
  return props;
}

describe("AiProviderSection", () => {
  it("sets the provider and applies its default model when a default exists", () => {
    const props = setup();
    fireEvent.change(screen.getByDisplayValue("Ollama (локальная)"), { target: { value: "claude" } });
    expect(props.setAiProvider).toHaveBeenCalledWith("claude");
    expect(props.setAiModel).toHaveBeenCalledWith("claude-opus-4-8");
  });

  it("offers Gemini and OpenRouter as provider options (#228)", () => {
    setup();
    const select = screen.getByDisplayValue("Ollama (локальная)") as HTMLSelectElement;
    const values = Array.from(select.options).map((o) => o.value);
    expect(values).toContain("gemini");
    expect(values).toContain("openrouter");
  });

  it("changes provider without touching the model when no default is registered", () => {
    const props = setup({ providerDefaults: {} });
    fireEvent.change(screen.getByDisplayValue("Ollama (локальная)"), { target: { value: "groq" } });
    expect(props.setAiProvider).toHaveBeenCalledWith("groq");
    expect(props.setAiModel).not.toHaveBeenCalled();
  });

  it("propagates model edits via the combobox free-text fallback (#229)", () => {
    const props = setup();
    // Model field is now a searchable combobox; the current value shows on
    // its trigger. Open it, type a custom model, commit with Enter.
    fireEvent.click(screen.getByText("gemma3:4b"));
    const search = screen.getByPlaceholderText(/Поиск модели/);
    fireEvent.change(search, { target: { value: "llama3" } });
    fireEvent.keyDown(search, { key: "Enter" });
    expect(props.setAiModel).toHaveBeenCalledWith("llama3");
  });

  it("renders the key input as password and toggles to text", () => {
    const props = setup({ showApiKey: false });
    const input = screen.getByPlaceholderText("Не задан");
    expect(input).toHaveAttribute("type", "password");
    // the eye button is the only icon-only button in the key row
    const eyeBtn = screen.getAllByRole("button").find((b) => b.textContent === "")!;
    fireEvent.click(eyeBtn);
    expect(props.setShowApiKey).toHaveBeenCalledWith(true);
  });

  it("shows the key as text when showApiKey is true and uses the masked placeholder", () => {
    setup({ showApiKey: true, maskedKey: "sk-••12" });
    const input = screen.getByPlaceholderText("sk-••12");
    expect(input).toHaveAttribute("type", "text");
  });

  it("enables the test button for ollama even without a key", () => {
    setup({ aiProvider: "ollama", aiApiKey: "", hasKey: false });
    expect(screen.getByRole("button", { name: /Проверить подключение/ })).not.toBeDisabled();
  });

  it("disables the test button for a cloud provider with no key", () => {
    setup({ aiProvider: "claude", aiApiKey: "", hasKey: false });
    expect(screen.getByRole("button", { name: /Проверить подключение/ })).toBeDisabled();
  });

  it("enables the cloud test button when a stored key exists", () => {
    setup({ aiProvider: "claude", aiApiKey: "", hasKey: true });
    expect(screen.getByRole("button", { name: /Проверить подключение/ })).not.toBeDisabled();
  });

  it("shows the progress label and disables the button while testing", () => {
    setup({ testing: true });
    expect(screen.getByRole("button", { name: /Проверяем подключение/ })).toBeDisabled();
  });

  it("fires onTest when clicked in a ready state", () => {
    const props = setup();
    fireEvent.click(screen.getByRole("button", { name: "Проверить подключение" }));
    expect(props.onTest).toHaveBeenCalledTimes(1);
  });

  it("shows the connected badge and an error banner", () => {
    setup({ active: true, testResult: { success: false, error: "401 Unauthorized" } });
    expect(screen.getByText("Подключен")).toBeInTheDocument();
    expect(screen.getByText("401 Unauthorized")).toBeInTheDocument();
  });
});
