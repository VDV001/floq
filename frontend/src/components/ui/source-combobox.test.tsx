import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { vi, describe, it, expect, beforeEach } from "vitest";
import { SourceCombobox } from "./source-combobox";

vi.mock("@/lib/api", () => ({
  api: {
    getSources: vi.fn(),
    createSource: vi.fn(),
    createSourceCategory: vi.fn(),
  },
}));

import { api } from "@/lib/api";

const mockCategories = [
  {
    id: "cat-1",
    name: "Organic",
    sort_order: 1,
    created_at: "2026-01-01",
    sources: [
      { id: "src-1", category_id: "cat-1", name: "Google", sort_order: 1, created_at: "2026-01-01" },
      { id: "src-2", category_id: "cat-1", name: "Referral", sort_order: 2, created_at: "2026-01-01" },
    ],
  },
  {
    id: "cat-2",
    name: "Paid",
    sort_order: 2,
    created_at: "2026-01-01",
    sources: [
      { id: "src-3", category_id: "cat-2", name: "Facebook Ads", sort_order: 1, created_at: "2026-01-01" },
    ],
  },
];

describe("SourceCombobox", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getSources).mockResolvedValue(mockCategories);
  });

  it("renders with placeholder", async () => {
    render(<SourceCombobox value={null} onChange={() => {}} />);
    await waitFor(() => {
      expect(screen.getByText("Выберите источник")).toBeInTheDocument();
    });
  });

  it("shows categories and sources on click", async () => {
    render(<SourceCombobox value={null} onChange={() => {}} />);

    await waitFor(() => {
      expect(api.getSources).toHaveBeenCalled();
    });

    fireEvent.click(screen.getByText("Выберите источник"));

    expect(screen.getByText("Organic")).toBeInTheDocument();
    expect(screen.getByText("Paid")).toBeInTheDocument();
    expect(screen.getByText("Google")).toBeInTheDocument();
    expect(screen.getByText("Referral")).toBeInTheDocument();
    expect(screen.getByText("Facebook Ads")).toBeInTheDocument();
  });

  it("calls onChange when selecting a source", async () => {
    const onChange = vi.fn();
    render(<SourceCombobox value={null} onChange={onChange} />);

    await waitFor(() => {
      expect(api.getSources).toHaveBeenCalled();
    });

    fireEvent.click(screen.getByText("Выберите источник"));
    fireEvent.click(screen.getByText("Google"));

    expect(onChange).toHaveBeenCalledWith("src-1");
  });

  it("creates a new source", async () => {
    const onChange = vi.fn();
    vi.mocked(api.createSource).mockResolvedValue({
      id: "src-new",
      category_id: "cat-1",
      name: "New Source",
      sort_order: 3,
      created_at: "2026-01-01",
    });

    render(<SourceCombobox value={null} onChange={onChange} />);

    await waitFor(() => {
      expect(api.getSources).toHaveBeenCalled();
    });

    fireEvent.click(screen.getByText("Выберите источник"));
    fireEvent.click(screen.getByText("Добавить источник"));

    const nameInput = screen.getByPlaceholderText("Название источника");
    fireEvent.change(nameInput, { target: { value: "New Source" } });
    fireEvent.click(screen.getByText("Создать"));

    await waitFor(() => {
      expect(api.createSource).toHaveBeenCalledWith("cat-1", "New Source");
      expect(onChange).toHaveBeenCalledWith("src-new");
    });
  });

  it("shows the selected source name and clears it via the X button", async () => {
    const onChange = vi.fn();
    const { container } = render(<SourceCombobox value="src-1" onChange={onChange} />);

    await waitFor(() => {
      expect(screen.getByText("Google")).toBeInTheDocument();
    });

    // Clearing must call onChange(null). The clear affordance is the span
    // carrying the hover:bg-slate-200 token wrapping the X icon.
    const clearBtn = container.querySelector<HTMLElement>("span.hover\\:bg-slate-200")!;
    expect(clearBtn).toBeTruthy();
    fireEvent.click(clearBtn);
    expect(onChange).toHaveBeenCalledWith(null);
  });

  it("filters sources by the search query", async () => {
    render(<SourceCombobox value={null} onChange={() => {}} />);
    await waitFor(() => expect(api.getSources).toHaveBeenCalled());

    fireEvent.click(screen.getByText("Выберите источник"));
    fireEvent.change(screen.getByPlaceholderText("Поиск..."), {
      target: { value: "google" },
    });

    expect(screen.getByText("Google")).toBeInTheDocument();
    expect(screen.queryByText("Referral")).not.toBeInTheDocument();
    expect(screen.queryByText("Facebook Ads")).not.toBeInTheDocument();
  });

  it("shows the empty state when nothing matches the search", async () => {
    render(<SourceCombobox value={null} onChange={() => {}} />);
    await waitFor(() => expect(api.getSources).toHaveBeenCalled());

    fireEvent.click(screen.getByText("Выберите источник"));
    fireEvent.change(screen.getByPlaceholderText("Поиск..."), {
      target: { value: "zzzz" },
    });

    expect(screen.getByText("Ничего не найдено")).toBeInTheDocument();
  });

  it("creates a source by pressing Enter in the name field", async () => {
    const onChange = vi.fn();
    vi.mocked(api.createSource).mockResolvedValue({
      id: "src-enter",
      category_id: "cat-1",
      name: "Via Enter",
      sort_order: 4,
      created_at: "2026-01-01",
    });

    render(<SourceCombobox value={null} onChange={onChange} />);
    await waitFor(() => expect(api.getSources).toHaveBeenCalled());

    fireEvent.click(screen.getByText("Выберите источник"));
    fireEvent.click(screen.getByText("Добавить источник"));

    const nameInput = screen.getByPlaceholderText("Название источника");
    fireEvent.change(nameInput, { target: { value: "Via Enter" } });
    fireEvent.keyDown(nameInput, { key: "Enter" });

    await waitFor(() => {
      expect(api.createSource).toHaveBeenCalledWith("cat-1", "Via Enter");
      expect(onChange).toHaveBeenCalledWith("src-enter");
    });
  });

  it("does not create when name or category is missing", async () => {
    render(<SourceCombobox value={null} onChange={() => {}} />);
    await waitFor(() => expect(api.getSources).toHaveBeenCalled());

    fireEvent.click(screen.getByText("Выберите источник"));
    fireEvent.click(screen.getByText("Добавить источник"));
    // Click Create with an empty name — must be a no-op.
    fireEvent.click(screen.getByText("Создать"));

    expect(api.createSource).not.toHaveBeenCalled();
  });

  it("lets the user pick a category in the create form", async () => {
    render(<SourceCombobox value={null} onChange={() => {}} />);
    await waitFor(() => expect(api.getSources).toHaveBeenCalled());

    fireEvent.click(screen.getByText("Выберите источник"));
    fireEvent.click(screen.getByText("Добавить источник"));

    const select = screen.getByRole("combobox");
    fireEvent.change(select, { target: { value: "cat-2" } });
    expect((select as HTMLSelectElement).value).toBe("cat-2");
  });

  it("cancels the create form and returns to the add button", async () => {
    render(<SourceCombobox value={null} onChange={() => {}} />);
    await waitFor(() => expect(api.getSources).toHaveBeenCalled());

    fireEvent.click(screen.getByText("Выберите источник"));
    fireEvent.click(screen.getByText("Добавить источник"));
    expect(screen.getByPlaceholderText("Название источника")).toBeInTheDocument();

    fireEvent.click(screen.getByText("Отмена"));
    expect(
      screen.queryByPlaceholderText("Название источника")
    ).not.toBeInTheDocument();
    expect(screen.getByText("Добавить источник")).toBeInTheDocument();
  });

  it("closes the dropdown on an outside click", async () => {
    render(
      <div>
        <SourceCombobox value={null} onChange={() => {}} />
        <button>outside</button>
      </div>
    );
    await waitFor(() => expect(api.getSources).toHaveBeenCalled());

    fireEvent.click(screen.getByText("Выберите источник"));
    expect(screen.getByPlaceholderText("Поиск...")).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByText("outside"));
    expect(screen.queryByPlaceholderText("Поиск...")).not.toBeInTheDocument();
  });

  it("clears the search and closes after selecting a filtered source", async () => {
    const onChange = vi.fn();
    render(<SourceCombobox value={null} onChange={onChange} />);
    await waitFor(() => expect(api.getSources).toHaveBeenCalled());

    fireEvent.click(screen.getByText("Выберите источник"));
    fireEvent.change(screen.getByPlaceholderText("Поиск..."), {
      target: { value: "face" },
    });
    fireEvent.click(screen.getByText("Facebook Ads"));

    expect(onChange).toHaveBeenCalledWith("src-3");
    // Dropdown closed.
    expect(screen.queryByPlaceholderText("Поиск...")).not.toBeInTheDocument();
  });
});
