import type { DragEvent } from "react";
import { NewDocumentIcon, OpenFolderIcon, UndoIcon } from "@/components/editor-icons";

interface WelcomeScreenProps {
  isDragOver: boolean;
  hasAutosave: boolean;
  onNew: () => void;
  onOpen: () => void;
  onResume: () => void;
  onDragOver: (event: DragEvent) => void;
  onDragLeave: (event: DragEvent) => void;
  onDrop: (event: DragEvent) => void;
}

export function WelcomeScreen({
  isDragOver,
  hasAutosave,
  onNew,
  onOpen,
  onResume,
  onDragOver,
  onDragLeave,
  onDrop,
}: WelcomeScreenProps) {
  return (
    <section
      aria-label="Welcome screen drop zone"
      className="relative flex h-full w-full items-center justify-center overflow-hidden p-6"
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      onDrop={onDrop}
    >
      <div className="pointer-events-none absolute top-[12%] left-[18%] h-56 w-56 rounded-full bg-accent/8 blur-3xl" />
      <div className="pointer-events-none absolute right-[16%] bottom-[10%] h-64 w-64 rounded-full bg-highlight/8 blur-3xl" />

      <div className="relative flex w-full max-w-[560px] flex-col items-center gap-5 overflow-hidden rounded-2xl border border-white/12 bg-[linear-gradient(145deg,hsl(var(--panel)/0.96),hsl(var(--panel-soft)/0.96))] p-8 shadow-[0_28px_80px_rgba(0,0,0,0.42)] backdrop-blur-xl">
        <div className="absolute inset-x-12 top-0 h-px bg-[linear-gradient(90deg,transparent,hsl(var(--accent)/0.8),hsl(var(--highlight)/0.6),transparent)]" />

        <div className="flex flex-col items-center gap-2.5 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-[linear-gradient(135deg,hsl(var(--accent)),hsl(var(--highlight)))] text-xl font-black text-slate-950 shadow-[0_0_32px_hsl(var(--accent)/0.2)]">
            A
          </div>
          <div className="mt-1 flex items-center gap-2">
            <span className="text-[10px] font-semibold tracking-[0.18em] text-accent uppercase">
              Agogo Studio
            </span>
            <span className="rounded-full border border-highlight/30 bg-highlight/10 px-2 py-0.5 text-[9px] font-medium text-purple-200">
              WASM powered
            </span>
          </div>
          <h1 className="text-2xl font-semibold tracking-[-0.02em] text-white">
            Make something remarkable.
          </h1>
          <p className="max-w-sm text-[13px] leading-5 text-muted-foreground">
            Start with a blank canvas or bring in an image to continue creating.
          </p>
        </div>

        <div
          className={[
            "group flex w-full flex-col items-center gap-2 rounded-xl border border-dashed px-5 py-6 transition-all",
            isDragOver
              ? "scale-[1.01] border-accent bg-accent/10 shadow-[0_0_30px_hsl(var(--accent)/0.08)]"
              : "border-muted-foreground/30 bg-black/12 hover:border-accent/45 hover:bg-accent/5",
          ].join(" ")}
        >
          <div className="mb-1 flex h-10 w-10 items-center justify-center rounded-full border border-white/10 bg-white/5 text-muted-foreground transition group-hover:border-accent/25 group-hover:text-accent">
            <UploadIcon className="h-5 w-5" />
          </div>
          <p className="text-[13px] font-medium text-foreground">Drop an image or project here</p>
          <p className="text-[11px] text-muted-foreground">PNG · JPEG · GIF · WebP · BMP · AGP</p>
        </div>

        <div className="grid w-full grid-cols-2 gap-2.5">
          <button
            type="button"
            onClick={onNew}
            className="flex h-10 items-center justify-center gap-2 rounded-[var(--ui-radius-md)] bg-accent px-4 text-[13px] font-semibold text-accent-foreground shadow-[0_8px_24px_hsl(var(--accent)/0.14)] transition hover:brightness-110 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <NewDocumentIcon className="h-4 w-4" />
            New Document
          </button>
          <button
            type="button"
            onClick={onOpen}
            className="flex h-10 items-center justify-center gap-2 rounded-[var(--ui-radius-md)] border border-border bg-panel-strong/70 px-4 text-[13px] font-semibold text-foreground transition hover:border-muted-foreground/50 hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <OpenFolderIcon className="h-4 w-4 text-accent" />
            Open File…
          </button>
          {hasAutosave && (
            <button
              type="button"
              onClick={onResume}
              className="col-span-2 flex items-center justify-center gap-2 rounded-[var(--ui-radius-md)] px-4 py-2 text-[12px] font-medium text-muted-foreground transition hover:bg-white/5 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              <UndoIcon className="h-3.5 w-3.5" />
              Resume last session
            </button>
          )}
        </div>
      </div>
    </section>
  );
}

function UploadIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.5}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
      <polyline points="17 8 12 3 7 8" />
      <line x1="12" y1="3" x2="12" y2="15" />
    </svg>
  );
}
