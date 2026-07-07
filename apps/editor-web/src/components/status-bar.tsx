import { useRef, useState } from "react";
import { DropdownMenuContent, DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { Separator } from "@/components/ui/separator";
import { useCursorState } from "@/state/view-state";
import { useEngine } from "@/wasm/context";
import { useUiMeta, useViewport } from "@/wasm/use-engine-render";

/**
 * The editor footer: document name/dimensions, cursor position, and the zoom
 * readout with its quick zoom-level menu.
 */
export function StatusBar({ documentName }: { documentName: string }) {
  const engine = useEngine();
  const viewport = useViewport();
  const documentSize = useUiMeta((meta) =>
    meta ? `${meta.documentWidth} x ${meta.documentHeight}` : "No document",
  );
  const { cursor } = useCursorState();
  const [zoomMenuOpen, setZoomMenuOpen] = useState(false);
  const zoomClickTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const zoomPercent = viewport ? `${Math.round(viewport.zoom * 100)}%` : "0%";
  const cursorText = cursor ? `${cursor.x}, ${cursor.y}` : "—";

  function handleZoomClick() {
    if (zoomClickTimerRef.current) return;
    zoomClickTimerRef.current = setTimeout(() => {
      zoomClickTimerRef.current = null;
      setZoomMenuOpen((prev) => !prev);
    }, 200);
  }

  function handleZoomDoubleClick() {
    if (zoomClickTimerRef.current) {
      clearTimeout(zoomClickTimerRef.current);
      zoomClickTimerRef.current = null;
    }
    engine.setZoom(1);
    setZoomMenuOpen(false);
  }

  return (
    <footer className="editor-footerbar flex h-[28px] items-center justify-between gap-3 border-t border-border px-2 text-[11px] text-muted-foreground/70">
      <div className="flex items-center gap-2 overflow-hidden">
        <span className="truncate text-muted-foreground">{documentName}.agp</span>
        <Separator orientation="vertical" className="h-3 bg-border" />
        <span>{documentSize}</span>
        <Separator orientation="vertical" className="h-3 bg-border" />
        <span>{cursorText}</span>
      </div>
      <div className="relative flex items-center gap-2">
        {zoomMenuOpen && (
          <>
            <button
              type="button"
              className="fixed inset-0 z-40"
              aria-label="Close zoom menu"
              onClick={() => setZoomMenuOpen(false)}
            />
            <DropdownMenuContent className="absolute bottom-full right-0 z-50 mb-1 min-w-[100px] rounded-xl p-1">
              {[25, 50, 75, 100, 150, 200, 300, 400].map((level) => (
                <DropdownMenuItem
                  key={level}
                  className={`text-[11px] py-1 px-3 rounded-lg ${Math.round((viewport?.zoom ?? 1) * 100) === level ? "text-accent" : ""}`}
                  onClick={() => {
                    engine.setZoom(level / 100);
                    setZoomMenuOpen(false);
                  }}
                >
                  {level}%
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </>
        )}
        <button
          type="button"
          className="cursor-pointer select-none tabular-nums text-foreground hover:text-white"
          onClick={handleZoomClick}
          onDoubleClick={handleZoomDoubleClick}
          title="Click for zoom options · Double-click to reset to 100%"
        >
          {zoomPercent}
        </button>
      </div>
    </footer>
  );
}
