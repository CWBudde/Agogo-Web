import type { DragEvent } from "react";

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
      className="flex h-full w-full items-center justify-center"
      onDragOver={onDragOver}
      onDragLeave={onDragLeave}
      onDrop={onDrop}
    >
      <div className="flex w-[480px] flex-col items-center gap-6 rounded-xl border border-white/8 bg-[#1a1d22] p-10 shadow-2xl">
        {/* Logo */}
        <div className="flex flex-col items-center gap-2">
          <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-cyan-400/95 text-xl font-black text-slate-950">
            A
          </div>
          <span className="font-serif text-lg font-semibold italic tracking-wide text-white">
            Agogo Studio
          </span>
          <p className="text-sm text-slate-400">Your creative workspace</p>
        </div>

        {/* Drop zone */}
        <div
          className={[
            "flex w-full flex-col items-center gap-3 rounded-lg border-2 border-dashed py-10 transition-colors",
            isDragOver
              ? "border-cyan-400 bg-cyan-400/5"
              : "border-white/12 bg-white/2",
          ].join(" ")}
        >
          <UploadIcon className="h-8 w-8 text-slate-500" />
          <p className="text-sm text-slate-400">
            Drop an image or project file
          </p>
          <p className="text-xs text-slate-600">PNG, JPEG, GIF, WebP, BMP, .agp</p>
        </div>

        {/* Action buttons */}
        <div className="flex w-full flex-col gap-2">
          <button
            type="button"
            onClick={onNew}
            className="w-full rounded-md bg-cyan-500 px-4 py-2 text-sm font-medium text-slate-950 transition hover:bg-cyan-400 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400"
          >
            New Document
          </button>
          <button
            type="button"
            onClick={onOpen}
            className="w-full rounded-md border border-white/12 bg-white/4 px-4 py-2 text-sm font-medium text-slate-200 transition hover:bg-white/8 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/20"
          >
            Open File…
          </button>
          {hasAutosave && (
            <button
              type="button"
              onClick={onResume}
              className="w-full rounded-md px-4 py-2 text-sm font-medium text-slate-400 transition hover:text-slate-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/20"
            >
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
