import { CommandID, type TextEditInputCommand } from "@agogo/proto";
import { useEffect, useRef } from "react";

export function TextEditOverlay({
  engine,
  initialText,
}: {
  engine: { dispatchCommand: (id: number, payload: unknown) => void };
  initialText: string;
}) {
  const inputRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    const el = inputRef.current;
    if (!el) return;
    el.focus();
    // Move cursor to end so the user can continue typing.
    el.setSelectionRange(el.value.length, el.value.length);
  }, []);

  return (
    <div className="editor-chrome flex min-h-[34px] items-center gap-3 border-b border-blue-500/30 bg-blue-500/8 px-3 py-1">
      <span className="text-[11px] text-blue-200">Editing text —</span>
      <textarea
        ref={inputRef}
        defaultValue={initialText}
        className="flex-1 resize-none rounded border border-blue-500/30 bg-slate-800/80 px-2 py-0.5 text-[12px] text-slate-200 placeholder-slate-500 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-blue-500"
        rows={1}
        placeholder="Type here..."
        onInput={(e) => {
          const target = e.target as HTMLTextAreaElement;
          engine.dispatchCommand(CommandID.TextEditInput, {
            text: target.value,
          } satisfies TextEditInputCommand);
        }}
        onKeyDown={(e) => {
          if (e.key === "Escape") {
            e.preventDefault();
            engine.dispatchCommand(CommandID.CancelTextEdit, {});
          }
        }}
      />
      <button
        type="button"
        className="shrink-0 rounded border border-blue-500/40 px-2 py-0.5 text-[11px] text-blue-300 hover:bg-blue-500/15 focus-visible:outline-none"
        onClick={() => engine.dispatchCommand(CommandID.CommitTextEdit, {})}
      >
        Done
      </button>
    </div>
  );
}
