import { describe, it, expect, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { StepPreview } from "./StepPreview";

vi.mock("@/lib/api", () => ({
  api: {
    previewMessage: vi.fn(),
  },
}));

import { api } from "@/lib/api";
const mockedApi = vi.mocked(api);

describe("StepPreview", () => {
  it("renders input and generate button", () => {
    render(<StepPreview channel="email" promptHint="intro" onClose={vi.fn()} />);

    expect(screen.getByDisplayValue("Иван Петров")).toBeInTheDocument();
    expect(screen.getByText("Сгенерировать")).toBeInTheDocument();
  });

  it("generates preview text on click", async () => {
    mockedApi.previewMessage.mockResolvedValue({ text: "Привет, Иван!" });

    render(<StepPreview channel="email" promptHint="intro" onClose={vi.fn()} />);

    await userEvent.click(screen.getByText("Сгенерировать"));

    await waitFor(() => {
      expect(screen.getByText("Привет, Иван!")).toBeInTheDocument();
    });
    expect(mockedApi.previewMessage).toHaveBeenCalledWith("Иван Петров", "", "", "email", "intro");
  });

  it("shows error on generation failure", async () => {
    mockedApi.previewMessage.mockRejectedValue(new Error("fail"));

    render(<StepPreview channel="telegram" promptHint="followup" onClose={vi.fn()} />);

    await userEvent.click(screen.getByText("Сгенерировать"));

    await waitFor(() => {
      expect(screen.getByText("Ошибка генерации")).toBeInTheDocument();
    });
  });

  it("calls onClose from cancel button", async () => {
    const onClose = vi.fn();
    render(<StepPreview channel="email" promptHint="intro" onClose={onClose} />);

    await userEvent.click(screen.getByText("Отмена"));

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("calls onClose from close link after generation", async () => {
    mockedApi.previewMessage.mockResolvedValue({ text: "Hello!" });
    const onClose = vi.fn();

    render(<StepPreview channel="email" promptHint="intro" onClose={onClose} />);

    await userEvent.click(screen.getByText("Сгенерировать"));
    await waitFor(() => expect(screen.getByText("Hello!")).toBeInTheDocument());

    await userEvent.click(screen.getByText("Закрыть"));

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("allows changing prospect name", async () => {
    mockedApi.previewMessage.mockResolvedValue({ text: "Hi!" });

    render(<StepPreview channel="email" promptHint="intro" onClose={vi.fn()} />);

    const input = screen.getByDisplayValue("Иван Петров");
    await userEvent.clear(input);
    await userEvent.type(input, "Мария");
    await userEvent.click(screen.getByText("Сгенерировать"));

    expect(mockedApi.previewMessage).toHaveBeenCalledWith("Мария", "", "", "email", "intro");
  });
});
