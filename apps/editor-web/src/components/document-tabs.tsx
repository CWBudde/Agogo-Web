import { CommandID } from "@agogo/proto";
import { useEffect } from "react";
import { FileImageIcon } from "@/components/editor-icons";
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
      className="editor-chrome flex min-h-9 shrink-0 overflow-x-auto border-b border-border bg-panel-soft"
    >
      {documents.map((document) => (
        <div
          key={document.id}
          role="tab"
          aria-selected={document.active}
          tabIndex={document.active ? 0 : -1}
          className={`group relative flex min-w-36 max-w-64 items-center gap-2 border-r border-border px-3 text-xs transition ${
            document.active
              ? "bg-background text-foreground after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:bg-accent"
              : "bg-panel text-muted-foreground hover:bg-panel-strong hover:text-foreground"
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
          <FileImageIcon className="h-3.5 w-3.5 shrink-0 text-accent/80" />
          <span className="truncate font-medium">
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
