import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

const mockPathname = vi.fn();
vi.mock("next/navigation", () => ({
  usePathname: () => mockPathname(),
}));

import { AnalyticsTabs } from "./AnalyticsTabs";

describe("AnalyticsTabs", () => {
  it("renders every tab as a link", () => {
    mockPathname.mockReturnValue("/analytics/cost");
    render(<AnalyticsTabs />);
    for (const label of ["Sequences", "Затраты", "Входящие", "Горячие лиды", "Конверсия"]) {
      expect(screen.getByRole("link", { name: label })).toBeInTheDocument();
    }
  });

  it("marks the active tab via aria-current-equivalent styling on the matching pathname", () => {
    mockPathname.mockReturnValue("/analytics/cost");
    render(<AnalyticsTabs />);
    const active = screen.getByRole("link", { name: "Затраты" });
    expect(active.className).toContain("text-slate-900");
    const inactive = screen.getByRole("link", { name: "Sequences" });
    expect(inactive.className).toContain("text-slate-500");
  });

  it("leaves no tab active when the pathname matches none", () => {
    mockPathname.mockReturnValue("/analytics/unknown");
    render(<AnalyticsTabs />);
    expect(screen.getByRole("link", { name: "Затраты" }).className).toContain("text-slate-500");
  });
});
