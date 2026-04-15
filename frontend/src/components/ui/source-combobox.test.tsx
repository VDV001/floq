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
});
