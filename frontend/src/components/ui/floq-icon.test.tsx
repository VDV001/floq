import { render } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { FloqIcon } from "./floq-icon";

function svgOf(container: HTMLElement): SVGSVGElement {
  const svg = container.querySelector("svg");
  expect(svg).not.toBeNull();
  return svg as unknown as SVGSVGElement;
}

describe("FloqIcon", () => {
  it("renders the circle variant by default with a 48 viewBox", () => {
    const { container } = render(<FloqIcon />);
    const svg = svgOf(container);
    expect(svg).toHaveAttribute("viewBox", "0 0 48 48");
    expect(svg).toHaveAttribute("fill", "none");
    // The blue brand circle is unique to the circle variant.
    expect(container.querySelector('circle[fill="#004ac6"]')).not.toBeNull();
  });

  it("renders the flat variant with a 32 viewBox and currentColor fill", () => {
    const { container } = render(<FloqIcon variant="flat" />);
    const svg = svgOf(container);
    expect(svg).toHaveAttribute("viewBox", "0 0 32 32");
    expect(svg).toHaveAttribute("fill", "currentColor");
    // No brand circle in the flat variant.
    expect(container.querySelector('circle[fill="#004ac6"]')).toBeNull();
  });

  it("applies the default size of 40 to width and height", () => {
    const { container } = render(<FloqIcon />);
    const svg = svgOf(container);
    expect(svg).toHaveAttribute("width", "40");
    expect(svg).toHaveAttribute("height", "40");
  });

  it("honors a custom size on both variants", () => {
    const circle = render(<FloqIcon size={64} />);
    expect(svgOf(circle.container)).toHaveAttribute("width", "64");

    const flat = render(<FloqIcon variant="flat" size={24} />);
    expect(svgOf(flat.container)).toHaveAttribute("height", "24");
  });

  it("forwards a custom className", () => {
    const { container } = render(<FloqIcon className="brand-mark" />);
    expect(svgOf(container)).toHaveClass("brand-mark");
  });
});
