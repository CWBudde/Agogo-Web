import { describe, expect, it } from "vitest";
import { cn, parseNumericInput } from "@/lib/utils";

describe("cn", () => {
  it("joins truthy class names in order", () => {
    expect(cn("panel", false, "active", null, undefined, "rounded")).toBe("panel active rounded");
  });

  it("returns an empty string when every value is falsy", () => {
    expect(cn(false, null, undefined)).toBe("");
  });
});

describe("parseNumericInput", () => {
  it("returns the fallback for an empty string", () => {
    expect(parseNumericInput("", 42)).toBe(42);
  });

  it("returns the fallback for whitespace-only input", () => {
    expect(parseNumericInput("  ", 7)).toBe(7);
  });

  it("returns the fallback for non-numeric input", () => {
    expect(parseNumericInput("abc", 13)).toBe(13);
  });

  it("parses integers", () => {
    expect(parseNumericInput("42", 0)).toBe(42);
  });

  it("parses negative decimals", () => {
    expect(parseNumericInput("-3.5", 0)).toBe(-3.5);
  });

  it("passes the fallback through untouched, including 0 and negatives", () => {
    expect(parseNumericInput("", 0)).toBe(0);
    expect(parseNumericInput("x", -12.25)).toBe(-12.25);
  });

  it("returns the fallback for non-finite input", () => {
    expect(parseNumericInput("Infinity", 5)).toBe(5);
  });
});
