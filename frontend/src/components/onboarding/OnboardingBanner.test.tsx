import type { ReactNode } from "react";
import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { OnboardingBanner, ONBOARDING_BANNER_HIDDEN_KEY } from "./OnboardingBanner";

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string; [k: string]: unknown }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

describe("OnboardingBanner", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it("renders the nudge linking to the onboarding tutorial", () => {
    render(<OnboardingBanner />);

    expect(screen.getByText("Впервые здесь? Пройдите обучение")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Открыть обучение/ })).toHaveAttribute("href", "/onboarding");
  });

  it("hides and persists the choice when dismissed", async () => {
    render(<OnboardingBanner />);

    await userEvent.click(screen.getByRole("button", { name: "Скрыть подсказку об обучении" }));

    expect(screen.queryByText("Впервые здесь? Пройдите обучение")).not.toBeInTheDocument();
    expect(window.localStorage.getItem(ONBOARDING_BANNER_HIDDEN_KEY)).toBe("1");
  });

  it("renders nothing when already dismissed/completed", () => {
    window.localStorage.setItem(ONBOARDING_BANNER_HIDDEN_KEY, "1");

    const { container } = render(<OnboardingBanner />);

    expect(container).toBeEmptyDOMElement();
  });
});
