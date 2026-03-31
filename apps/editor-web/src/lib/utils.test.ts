import { describe, expect, it } from "vitest";
import { cn } from "@/lib/utils";

describe("cn", () => {
  it("joins truthy class names in order", () => {
    expect(cn("panel", false, "active", null, undefined, "rounded")).toBe(
      "panel active rounded",
    );
  });

  it("returns an empty string when every value is falsy", () => {
    expect(cn(false, null, undefined)).toBe("");
  });
});
