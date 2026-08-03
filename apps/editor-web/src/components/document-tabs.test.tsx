import { CommandID, type DocumentSummary } from "@agogo/proto";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DocumentTabs } from "./document-tabs";

const dispatchCommand = vi.fn();
let documents: DocumentSummary[] = [];

vi.mock("@/wasm/context", () => ({
  useEngine: () => ({ dispatchCommand }),
}));

vi.mock("@/wasm/use-engine-render", () => ({
  useUiMeta: (selector: (meta: { documents: DocumentSummary[] }) => unknown) =>
    selector({ documents }),
}));

describe("DocumentTabs", () => {
  beforeEach(() => {
    dispatchCommand.mockReset();
    documents = [
      { id: "one", name: "One", width: 10, height: 10, active: true, modified: false },
      { id: "two", name: "Two", width: 20, height: 20, active: false, modified: true },
    ];
  });

  it("switches tabs and exposes the modified indicator", () => {
    render(<DocumentTabs />);
    fireEvent.click(screen.getByRole("tab", { name: /Two/ }));
    expect(dispatchCommand).toHaveBeenCalledWith(CommandID.SwitchDocument, { documentId: "two" });
    expect(screen.getByText("Two •")).not.toBeNull();
  });

  it("cycles documents with Ctrl+Tab", () => {
    render(<DocumentTabs />);
    fireEvent.keyDown(window, { key: "Tab", ctrlKey: true });
    expect(dispatchCommand).toHaveBeenCalledWith(CommandID.SwitchDocument, { documentId: "two" });
  });

  it("cycles backwards with Ctrl+Shift+Tab and wraps at the first tab", () => {
    render(<DocumentTabs />);
    fireEvent.keyDown(window, { key: "Tab", ctrlKey: true, shiftKey: true });
    expect(dispatchCommand).toHaveBeenCalledWith(CommandID.SwitchDocument, { documentId: "two" });
  });

  it("supports the platform Meta shortcut and ignores unrelated key presses", () => {
    render(<DocumentTabs />);
    fireEvent.keyDown(window, { key: "Tab", metaKey: true });
    fireEvent.keyDown(window, { key: "Tab" });
    fireEvent.keyDown(window, { key: "Enter", ctrlKey: true });
    expect(dispatchCommand).toHaveBeenCalledTimes(1);
    expect(dispatchCommand).toHaveBeenCalledWith(CommandID.SwitchDocument, { documentId: "two" });
  });

  it("switches a focused tab with the keyboard", () => {
    render(<DocumentTabs />);
    const secondTab = screen.getByRole("tab", { name: /Two/ });
    fireEvent.keyDown(secondTab, { key: " " });
    expect(dispatchCommand).toHaveBeenCalledWith(CommandID.SwitchDocument, { documentId: "two" });
  });

  it("confirms before closing a modified document", () => {
    const confirm = vi.spyOn(window, "confirm").mockReturnValue(false);
    render(<DocumentTabs />);
    fireEvent.click(screen.getByRole("button", { name: "Close Two" }));
    expect(confirm).toHaveBeenCalled();
    expect(dispatchCommand).not.toHaveBeenCalled();
    confirm.mockRestore();
  });

  it("closes clean documents without confirmation and does not also switch tabs", () => {
    const confirm = vi.spyOn(window, "confirm");
    render(<DocumentTabs />);
    fireEvent.click(screen.getByRole("button", { name: "Close One" }));
    expect(confirm).not.toHaveBeenCalled();
    expect(dispatchCommand).toHaveBeenCalledTimes(1);
    expect(dispatchCommand).toHaveBeenCalledWith(CommandID.CloseDocument, { documentId: "one" });
  });

  it("closes a document with the middle mouse button", () => {
    documents[1] = { ...documents[1], modified: false };
    render(<DocumentTabs />);
    fireEvent.mouseDown(screen.getByRole("tab", { name: /Two/ }), { button: 1 });
    expect(dispatchCommand).toHaveBeenCalledWith(CommandID.CloseDocument, { documentId: "two" });
  });

  it("renders no tab bar when no documents are open", () => {
    documents = [];
    render(<DocumentTabs />);
    expect(screen.queryByRole("tablist")).toBeNull();
  });
});
