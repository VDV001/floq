import type { ReactNode } from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Sequence, SequenceStep } from "@/lib/api";
import SequencesPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), back: vi.fn() }),
  usePathname: () => "/sequences",
}));

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string; [key: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

vi.mock("@/components/ui/switch", () => ({
  Switch: ({ checked, onCheckedChange, ...props }: { checked: boolean; onCheckedChange: (v: boolean) => void; [key: string]: unknown }) => (
    <button
      role="switch"
      aria-checked={checked}
      onClick={() => onCheckedChange?.(!checked)}
      {...props}
    />
  ),
}));

vi.mock("@/components/ui/separator", () => ({
  Separator: (props: React.HTMLAttributes<HTMLHRElement>) => <hr {...props} />,
}));

const mockSequences = [
  {
    id: "seq-1",
    user_id: "u1",
    name: "Холодная рассылка",
    is_active: true,
    created_at: "2026-01-01T00:00:00Z",
  },
  {
    id: "seq-2",
    user_id: "u1",
    name: "Фоллоуап",
    is_active: false,
    created_at: "2026-01-02T00:00:00Z",
  },
];

vi.mock("@/lib/api", () => ({
  api: {
    getSequences: vi.fn(),
    getSequence: vi.fn(),
    createSequence: vi.fn(),
    deleteSequence: vi.fn(),
    updateSequence: vi.fn(),
    addStep: vi.fn(),
    deleteStep: vi.fn(),
    toggleSequence: vi.fn(),
    previewMessage: vi.fn(),
    launchSequence: vi.fn(),
    getProspects: vi.fn(),
  },
}));

import { api } from "@/lib/api";

describe("SequencesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getSequences).mockResolvedValue(mockSequences as Sequence[]);
    vi.mocked(api.getSequence).mockResolvedValue({
      sequence: mockSequences[0],
      steps: [],
    } as { sequence: Sequence; steps: SequenceStep[] });
    vi.mocked(api.getProspects).mockResolvedValue([]);
  });

  it("renders sequence list after loading", async () => {
    render(<SequencesPage />);

    await waitFor(() => {
      expect(screen.getByText("Холодная рассылка")).toBeInTheDocument();
      expect(screen.getByText("Фоллоуап")).toBeInTheDocument();
    });
  });

  it("renders page header", async () => {
    render(<SequencesPage />);

    await waitFor(() => {
      expect(screen.getByText("Секвенции")).toBeInTheDocument();
    });
  });

  it("shows create button", async () => {
    render(<SequencesPage />);

    await waitFor(() => {
      expect(screen.getByText("Новая секвенция")).toBeInTheDocument();
    });
  });

  it("creates a new sequence", async () => {
    const user = userEvent.setup();
    vi.mocked(api.createSequence).mockResolvedValue({
      id: "seq-3",
      user_id: "u1",
      name: "Новая",
      is_active: false,
      created_at: "2026-01-03T00:00:00Z",
    } as Sequence);

    render(<SequencesPage />);

    await waitFor(() => {
      expect(screen.getByText("Секвенции")).toBeInTheDocument();
    });

    // Click the "Новая секвенция" button to open the inline form
    const newBtn = screen.getByText("Новая секвенция");
    await user.click(newBtn);

    // The inline form should appear with an input for the name
    const nameInput = await screen.findByPlaceholderText("Название секвенции...");
    await user.type(nameInput, "Новая");

    // Submit the form
    const createBtn = screen.getByText("Создать");
    await user.click(createBtn);

    await waitFor(() => {
      expect(api.createSequence).toHaveBeenCalledWith("Новая");
    });
  });
});
