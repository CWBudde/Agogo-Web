import { Button } from "@/components/ui/button";

interface EngineLoadErrorScreenProps {
  message: string;
}

/**
 * Shown in place of the welcome screen when the Wasm engine failed to load
 * (`engine.status === "error"`) — without it the app would render dead
 * buttons that silently do nothing.
 */
export function EngineLoadErrorScreen({ message }: EngineLoadErrorScreenProps) {
  return (
    <section
      aria-label="Engine load error"
      className="flex h-full w-full items-center justify-center"
    >
      <div className="editor-popup flex w-[420px] flex-col items-center gap-4 rounded-[var(--ui-radius-lg)] p-8 text-center">
        <h2 className="text-base font-semibold text-foreground">Engine failed to load</h2>
        <p className="break-words text-sm text-muted-foreground">{message}</p>
        <Button size="md" onClick={() => window.location.reload()}>
          Reload
        </Button>
      </div>
    </section>
  );
}
