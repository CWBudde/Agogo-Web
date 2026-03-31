import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ErrorBoundary } from "@/components/error-boundary";

function ThrowingChild() {
  throw new Error("boom");
}

describe("ErrorBoundary", () => {
  it("renders its children when no error occurs", () => {
    render(
      <ErrorBoundary>
        <div>editor shell</div>
      </ErrorBoundary>,
    );

    expect(screen.getByText("editor shell")).toBeTruthy();
  });

  it("renders the fallback UI when a child throws", () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});

    render(
      <ErrorBoundary>
        <ThrowingChild />
      </ErrorBoundary>,
    );

    expect(screen.getByText("The editor shell crashed")).toBeTruthy();
    expect(screen.getByText("boom")).toBeTruthy();

    consoleError.mockRestore();
  });
});
