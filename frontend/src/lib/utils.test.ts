import { describe, it, expect } from "vitest";
import { cn } from "./utils";

describe("cn", () => {
  it("returns empty string for no arguments", () => {
    expect(cn()).toBe("");
  });

  it("returns a single class", () => {
    expect(cn("px-4")).toBe("px-4");
  });

  it("merges multiple classes", () => {
    expect(cn("px-4", "py-2", "text-sm")).toBe("px-4 py-2 text-sm");
  });

  it("handles conditional classes", () => {
    expect(cn("px-4", false && "hidden", "py-2")).toBe("px-4 py-2");
    expect(cn("px-4", undefined, null, "py-2")).toBe("px-4 py-2");
  });

  it("resolves conflicting tailwind classes (last wins)", () => {
    expect(cn("px-4", "px-8")).toBe("px-8");
    expect(cn("text-red-500", "text-blue-500")).toBe("text-blue-500");
    expect(cn("bg-white", "bg-black")).toBe("bg-black");
  });

  it("handles array inputs", () => {
    expect(cn(["px-4", "py-2"])).toBe("px-4 py-2");
  });

  it("handles object inputs", () => {
    expect(cn({ "px-4": true, hidden: false, "py-2": true })).toBe("px-4 py-2");
  });

  it("merges complex conflicting classes", () => {
    expect(cn("p-4", "p-2")).toBe("p-2");
    expect(cn("mt-2 mb-4", "mt-6")).toBe("mb-4 mt-6");
  });
});
