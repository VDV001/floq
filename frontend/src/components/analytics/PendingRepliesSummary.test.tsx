import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import type { InboxFlowResponse } from "@/lib/api";
import { PendingRepliesSummary, formatDecideDuration } from "./PendingRepliesSummary";

type PR = InboxFlowResponse["pending_replies"];

function pr(over: Partial<PR> = {}): PR {
  return {
    approved: 80,
    rejected: 10,
    currently_pending: 5,
    approve_rate: 0.842,
    p50_time_to_decide_seconds: 120,
    p95_time_to_decide_seconds: 600,
    ...over,
  };
}

describe("formatDecideDuration", () => {
  it("formats sub-minute as seconds", () => {
    expect(formatDecideDuration(45)).toBe("45 с");
  });
  it("formats whole minutes", () => {
    expect(formatDecideDuration(120)).toBe("2 мин");
    expect(formatDecideDuration(600)).toBe("10 мин");
  });
  it("formats minutes with remainder seconds", () => {
    expect(formatDecideDuration(90)).toBe("1 мин 30 с");
  });
  it("renders a dash for zero (no decided rows)", () => {
    expect(formatDecideDuration(0)).toBe("—");
  });
});

describe("PendingRepliesSummary", () => {
  it("shows the approve rate as a rounded percentage", () => {
    render(<PendingRepliesSummary stats={pr()} />);
    expect(screen.getByText("84%")).toBeInTheDocument();
  });

  it("shows approved / rejected / pending counts", () => {
    render(<PendingRepliesSummary stats={pr()} />);
    expect(screen.getByTestId("pr-approved")).toHaveTextContent("80");
    expect(screen.getByTestId("pr-rejected")).toHaveTextContent("10");
    expect(screen.getByTestId("pr-pending")).toHaveTextContent("5");
  });

  it("shows the time-to-decide percentiles formatted", () => {
    render(<PendingRepliesSummary stats={pr()} />);
    expect(screen.getByTestId("pr-p50")).toHaveTextContent("2 мин");
    expect(screen.getByTestId("pr-p95")).toHaveTextContent("10 мин");
  });

  it("handles a zero-decision queue without NaN", () => {
    render(<PendingRepliesSummary stats={pr({ approved: 0, rejected: 0, approve_rate: 0, p50_time_to_decide_seconds: 0, p95_time_to_decide_seconds: 0 })} />);
    expect(screen.getByText("0%")).toBeInTheDocument();
    expect(screen.getByTestId("pr-p50")).toHaveTextContent("—");
  });
});
