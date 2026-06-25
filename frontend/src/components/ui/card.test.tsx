import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import {
  Card,
  CardHeader,
  CardFooter,
  CardTitle,
  CardAction,
  CardDescription,
  CardContent,
} from "./card";

describe("Card primitives", () => {
  it.each([
    ["Card", Card, "card"],
    ["CardHeader", CardHeader, "card-header"],
    ["CardTitle", CardTitle, "card-title"],
    ["CardDescription", CardDescription, "card-description"],
    ["CardAction", CardAction, "card-action"],
    ["CardContent", CardContent, "card-content"],
    ["CardFooter", CardFooter, "card-footer"],
  ] as const)("%s renders children with its data-slot", (label, Comp, slot) => {
    render(<Comp>{`${label} body`}</Comp>);
    const el = screen.getByText(`${label} body`);
    expect(el).toBeInTheDocument();
    expect(el).toHaveAttribute("data-slot", slot);
  });

  it("Card defaults data-size to 'default'", () => {
    render(<Card>def</Card>);
    expect(screen.getByText("def")).toHaveAttribute("data-size", "default");
  });

  it("Card honors the sm size", () => {
    render(<Card size="sm">small</Card>);
    expect(screen.getByText("small")).toHaveAttribute("data-size", "sm");
  });

  it("merges custom className onto a card slot", () => {
    render(<CardContent className="px-extra">c</CardContent>);
    expect(screen.getByText("c").className).toContain("px-extra");
  });

  it("composes a full card tree", () => {
    render(
      <Card>
        <CardHeader>
          <CardTitle>Title</CardTitle>
          <CardDescription>Desc</CardDescription>
          <CardAction>Act</CardAction>
        </CardHeader>
        <CardContent>Content</CardContent>
        <CardFooter>Footer</CardFooter>
      </Card>
    );
    expect(screen.getByText("Title")).toBeInTheDocument();
    expect(screen.getByText("Desc")).toBeInTheDocument();
    expect(screen.getByText("Act")).toBeInTheDocument();
    expect(screen.getByText("Content")).toBeInTheDocument();
    expect(screen.getByText("Footer")).toBeInTheDocument();
  });
});
