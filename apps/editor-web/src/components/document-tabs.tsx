import { CommandID } from "@agogo/proto";
import { useEffect } from "react";
import { useEngine } from "@/wasm/context";
import { useUiMeta } from "@/wasm/use-engine-render";

export function DocumentTabs() {
  const engine = useEngine();
  const documents = useUiMeta((meta) => meta?.documents ?? []);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (!(event.ctrlKey || event.metaKey) || event.key !== "Tab" || documents.length < 2) {
        return;
      }
      event.preventDefault();
      const activeIndex = Math.max(
        0,
        documents.findIndex((document) => document.active),
      );
      const delta = event.shiftKey ? -1 : 1;
      const next = (activeIndex + delta + documents.length) % documents.length;
      engine.dispatchCommand(CommandID.SwitchDocument, { documentId: documents[next].id });
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [documents, engine]);

  if (documents.length === 0) {
    return null;
  }

  const closeDocument = (documentId: string, name: string, modified: boolean) => {
    if (modified && !window.confirm(`Close ${name} without saving?`)) {
      return;
    }
    engine.dispatchCommand(CommandID.CloseDocument, { documentId });
  };

  return (
    <div
      role="tablist"
      aria-label="Open documents"
      className="editor-chrome flex min-h-8 overflow-x-auto border-b border-border bg-panel"
    >
      {documents.map((document) => (
        <div
          key={document.id}
          role="tab"
          aria-selected={document.active}
          tabIndex={document.active ? 0 : -1}
          className={`group flex min-w-32 max-w-64 items-center border-r border-border px-2 text-xs ${
            document.active ? "bg-background text-foreground" : "bg-panel text-muted-foreground"
          }`}
          onClick={() => {
            if (!document.active) {
              engine.dispatchCommand(CommandID.SwitchDocument, { documentId: document.id });
            }
          }}
          onMouseDown={(event) => {
            if (event.button === 1) {
              event.preventDefault();
              closeDocument(document.id, document.name, document.modified);
            }
          }}
          onKeyDown={(event) => {
            if (event.key === "Enter" || event.key === " ") {
              event.preventDefault();
              engine.dispatchCommand(CommandID.SwitchDocument, { documentId: document.id });
            }
          }}
        >
          <span className="truncate">
            {document.name}
            {document.modified ? " •" : ""}
          </span>
          <button
            type="button"
            className="ml-auto rounded px-1 text-muted-foreground hover:bg-muted hover:text-foreground"
            aria-label={`Close ${document.name}`}
            onClick={(event) => {
              event.stopPropagation();
              closeDocument(document.id, document.name, document.modified);
            }}
          >
            ×
          </button>
        </div>
      ))}
    </div>
  );
}
