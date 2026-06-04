import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { LeadsByChannelCard } from "./LeadsByChannelCard";

describe("LeadsByChannelCard", () => {
  it("renders each channel with its count and share", () => {
    render(<LeadsByChannelCard byChannel={{ telegram: 70, email: 30 }} total={100} />);
    expect(screen.getByTestId("channel-telegram")).toHaveTextContent("70");
    expect(screen.getByTestId("channel-email")).toHaveTextContent("30");
    expect(screen.getByTestId("channel-telegram")).toHaveTextContent("70%");
  });

  it("shows a zero count for a channel absent from the map", () => {
    render(<LeadsByChannelCard byChannel={{ telegram: 5 }} total={5} />);
    expect(screen.getByTestId("channel-email")).toHaveTextContent("0");
  });

  it("renders an empty state when there are no leads", () => {
    render(<LeadsByChannelCard byChannel={{}} total={0} />);
    expect(screen.getByText(/Нет лидов/i)).toBeInTheDocument();
  });
});
