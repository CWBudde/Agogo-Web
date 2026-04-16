import { type PointerEvent, useCallback, useRef, useState } from "react";
import { cn } from "@/lib/utils";

export type BlendIfSliderValue = [number, number, number, number];

export interface BlendIfSliderProps {
  value: BlendIfSliderValue;
  onChange: (next: BlendIfSliderValue) => void;
  label?: string;
  className?: string;
}

// Which index of the value tuple is being dragged:
//   0 = lowHard, 1 = lowSoft, 2 = highSoft, 3 = highHard
// When alt is NOT held, dragging a handle at the low end moves 0 and 1 together,
// and dragging the high handle moves 2 and 3 together. Alt-drag moves only the
// soft half (index 1 or 2) independently, splitting the handle.
type DragTarget =
  | { kind: "low"; splitIndex: null | 0 | 1 }
  | { kind: "high"; splitIndex: null | 2 | 3 };

const TRACK_DOMAIN = 255;

// Shift-drag quantises movement to steps of 16. Photoshop doesn't snap these
// sliders but the task brief asks for "something useful"; this feels
// predictable and matches the coarse feel of Photoshop's level-style dialogs.
const SHIFT_STEP = 16;

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

function clampOrdering(next: BlendIfSliderValue): BlendIfSliderValue {
  const lowHard = clamp(Math.round(next[0]), 0, TRACK_DOMAIN);
  const lowSoft = clamp(Math.round(next[1]), lowHard, TRACK_DOMAIN);
  const highSoft = clamp(Math.round(next[2]), lowSoft, TRACK_DOMAIN);
  const highHard = clamp(Math.round(next[3]), highSoft, TRACK_DOMAIN);
  return [lowHard, lowSoft, highSoft, highHard];
}

function toPercent(value: number): string {
  return `${(clamp(value, 0, TRACK_DOMAIN) / TRACK_DOMAIN) * 100}%`;
}

export function BlendIfSlider({
  value,
  onChange,
  label,
  className,
}: BlendIfSliderProps): JSX.Element {
  const trackRef = useRef<HTMLDivElement | null>(null);
  const [drag, setDrag] = useState<DragTarget | null>(null);

  const [lowHard, lowSoft, highSoft, highHard] = value;

  const positionToValue = useCallback((clientX: number): number => {
    const track = trackRef.current;
    if (!track) return 0;
    const rect = track.getBoundingClientRect();
    if (rect.width <= 0) return 0;
    const ratio = (clientX - rect.left) / rect.width;
    return clamp(ratio * TRACK_DOMAIN, 0, TRACK_DOMAIN);
  }, []);

  const applyDrag = useCallback(
    (target: DragTarget, clientX: number, shiftKey: boolean, altKey: boolean) => {
      const raw = positionToValue(clientX);
      const stepped = shiftKey
        ? Math.round(raw / SHIFT_STEP) * SHIFT_STEP
        : Math.round(raw);
      const next: BlendIfSliderValue = [value[0], value[1], value[2], value[3]];

      if (target.kind === "low") {
        if (altKey || target.splitIndex !== null) {
          // Split: move only the soft side (index 1). If user started a non-
          // split drag and now holds Alt, commit to splitIndex 1 from here on.
          const splitIndex = target.splitIndex ?? 1;
          if (splitIndex === 0) {
            next[0] = stepped;
            if (next[0] > next[1]) next[1] = next[0];
          } else {
            next[1] = stepped;
            if (next[1] < next[0]) next[0] = next[1];
          }
        } else {
          // Whole handle: preserve the gap between lowHard and lowSoft.
          const gap = value[1] - value[0];
          const newLowHard = clamp(stepped, 0, TRACK_DOMAIN - gap);
          next[0] = newLowHard;
          next[1] = newLowHard + gap;
        }
      } else {
        if (altKey || target.splitIndex !== null) {
          const splitIndex = target.splitIndex ?? 2;
          if (splitIndex === 3) {
            next[3] = stepped;
            if (next[3] < next[2]) next[2] = next[3];
          } else {
            next[2] = stepped;
            if (next[2] > next[3]) next[3] = next[2];
          }
        } else {
          const gap = value[3] - value[2];
          const newHighSoft = clamp(stepped, gap, TRACK_DOMAIN);
          next[2] = newHighSoft;
          next[3] = newHighSoft + gap;
        }
      }

      // After adjusting the dragged side, push neighbours so the ordering
      // invariant holds globally (lowHard ≤ lowSoft ≤ highSoft ≤ highHard).
      if (target.kind === "low") {
        if (next[1] > next[2]) next[2] = next[1];
        if (next[2] > next[3]) next[3] = next[2];
      } else {
        if (next[2] < next[1]) next[1] = next[2];
        if (next[1] < next[0]) next[0] = next[1];
      }

      onChange(clampOrdering(next));
    },
    [onChange, positionToValue, value],
  );

  const handlePointerDown = useCallback(
    (target: DragTarget) => (event: PointerEvent<HTMLDivElement>) => {
      if (event.button !== 0) return;
      event.preventDefault();
      event.stopPropagation();
      event.currentTarget.setPointerCapture(event.pointerId);

      // Resolve split target on the initial press: Alt at press-time commits
      // to dragging only the soft half.
      const resolved: DragTarget =
        target.kind === "low"
          ? { kind: "low", splitIndex: event.altKey ? 1 : null }
          : { kind: "high", splitIndex: event.altKey ? 2 : null };

      setDrag(resolved);
      applyDrag(resolved, event.clientX, event.shiftKey, event.altKey);
    },
    [applyDrag],
  );

  const handlePointerMove = useCallback(
    (event: PointerEvent<HTMLDivElement>) => {
      if (!drag) return;
      // If user starts a non-split drag then holds Alt, promote it mid-drag
      // to split dragging the soft side.
      let effective: DragTarget = drag;
      if (event.altKey && drag.splitIndex === null) {
        effective =
          drag.kind === "low"
            ? { kind: "low", splitIndex: 1 }
            : { kind: "high", splitIndex: 2 };
        setDrag(effective);
      }
      applyDrag(effective, event.clientX, event.shiftKey, event.altKey);
    },
    [applyDrag, drag],
  );

  const handlePointerUp = useCallback(
    (event: PointerEvent<HTMLDivElement>) => {
      if (!drag) return;
      event.currentTarget.releasePointerCapture?.(event.pointerId);
      setDrag(null);
    },
    [drag],
  );

  const lowSplit = lowHard !== lowSoft;
  const highSplit = highSoft !== highHard;

  return (
    <div className={cn("flex flex-col gap-1", className)}>
      {label ? (
        <div className="text-[11px] text-slate-400">{label}</div>
      ) : null}
      <div
        ref={trackRef}
        role="slider"
        tabIndex={0}
        aria-label={label ?? "Blend If range"}
        aria-valuemin={0}
        aria-valuemax={TRACK_DOMAIN}
        aria-valuenow={lowSoft}
        aria-valuetext={`${lowHard}/${lowSoft} — ${highSoft}/${highHard}`}
        data-testid="blend-if-slider-track"
        className="relative h-6 rounded border border-white/10 bg-slate-800 select-none"
      >
        {/* Fade zone: left (lowHard..lowSoft) */}
        <div
          className="pointer-events-none absolute top-0 bottom-0 bg-gradient-to-r from-slate-700/0 to-slate-400/60"
          style={{
            left: toPercent(lowHard),
            width: `calc(${toPercent(lowSoft)} - ${toPercent(lowHard)})`,
          }}
        />
        {/* Pass-through zone (lowSoft..highSoft) */}
        <div
          className="pointer-events-none absolute top-0 bottom-0 bg-slate-300/70"
          style={{
            left: toPercent(lowSoft),
            width: `calc(${toPercent(highSoft)} - ${toPercent(lowSoft)})`,
          }}
        />
        {/* Fade zone: right (highSoft..highHard) */}
        <div
          className="pointer-events-none absolute top-0 bottom-0 bg-gradient-to-r from-slate-400/60 to-slate-700/0"
          style={{
            left: toPercent(highSoft),
            width: `calc(${toPercent(highHard)} - ${toPercent(highSoft)})`,
          }}
        />

        {/* Low handle */}
        <HandleVisual
          split={lowSplit}
          side="low"
          hardPercent={toPercent(lowHard)}
          softPercent={toPercent(lowSoft)}
          onPointerDown={handlePointerDown({ kind: "low", splitIndex: null })}
          onPointerMove={handlePointerMove}
          onPointerUp={handlePointerUp}
          onPointerCancel={handlePointerUp}
          testId="blend-if-handle-low"
        />

        {/* High handle */}
        <HandleVisual
          split={highSplit}
          side="high"
          hardPercent={toPercent(highHard)}
          softPercent={toPercent(highSoft)}
          onPointerDown={handlePointerDown({ kind: "high", splitIndex: null })}
          onPointerMove={handlePointerMove}
          onPointerUp={handlePointerUp}
          onPointerCancel={handlePointerUp}
          testId="blend-if-handle-high"
        />
      </div>
      <div
        className="flex justify-between text-[10px] text-slate-500 tabular-nums"
        data-testid="blend-if-readout"
      >
        <span>
          {lowSplit ? `${lowHard} / ${lowSoft}` : `${lowHard}`}
        </span>
        <span>
          {highSplit ? `${highSoft} / ${highHard}` : `${highHard}`}
        </span>
      </div>
    </div>
  );
}

interface HandleVisualProps {
  split: boolean;
  side: "low" | "high";
  hardPercent: string;
  softPercent: string;
  onPointerDown: (event: PointerEvent<HTMLDivElement>) => void;
  onPointerMove: (event: PointerEvent<HTMLDivElement>) => void;
  onPointerUp: (event: PointerEvent<HTMLDivElement>) => void;
  onPointerCancel: (event: PointerEvent<HTMLDivElement>) => void;
  testId: string;
}

function HandleVisual({
  split,
  side,
  hardPercent,
  softPercent,
  onPointerDown,
  onPointerMove,
  onPointerUp,
  onPointerCancel,
  testId,
}: HandleVisualProps): JSX.Element {
  // When unified we render a single 10px-wide hit target; when split we render
  // two half-width hit targets centered on the hard and soft values.
  if (!split) {
    return (
      <div
        data-testid={testId}
        data-split="false"
        data-side={side}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerCancel}
        className="absolute top-0 bottom-0 w-3 -translate-x-1/2 cursor-ew-resize"
        style={{ left: hardPercent }}
      >
        <div className="absolute inset-y-0 left-1/2 w-[2px] -translate-x-1/2 bg-slate-100" />
        <div className="absolute bottom-0 left-1/2 h-2 w-2 -translate-x-1/2 translate-y-1 rotate-45 bg-slate-100" />
      </div>
    );
  }

  return (
    <>
      <div
        data-testid={`${testId}-hard`}
        data-split="true"
        data-side={side}
        data-half="hard"
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerCancel}
        className="absolute top-0 bottom-0 w-3 -translate-x-1/2 cursor-ew-resize"
        style={{ left: hardPercent }}
      >
        <div className="absolute inset-y-0 left-1/2 w-[2px] -translate-x-1/2 bg-slate-100" />
        <div
          className={cn(
            "absolute bottom-0 h-2 w-2 translate-y-1 bg-slate-100",
            side === "low" ? "left-1/2 origin-bottom-left" : "right-1/2 origin-bottom-right",
          )}
          style={{
            clipPath:
              side === "low"
                ? "polygon(0 0, 100% 100%, 0 100%)"
                : "polygon(100% 0, 100% 100%, 0 100%)",
          }}
        />
      </div>
      <div
        data-testid={`${testId}-soft`}
        data-split="true"
        data-side={side}
        data-half="soft"
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerCancel}
        className="absolute top-0 bottom-0 w-3 -translate-x-1/2 cursor-ew-resize"
        style={{ left: softPercent }}
      >
        <div className="absolute inset-y-0 left-1/2 w-[2px] -translate-x-1/2 bg-slate-300/60" />
        <div
          className={cn(
            "absolute bottom-0 h-2 w-2 translate-y-1 bg-slate-300",
            side === "low" ? "right-1/2 origin-bottom-right" : "left-1/2 origin-bottom-left",
          )}
          style={{
            clipPath:
              side === "low"
                ? "polygon(100% 0, 100% 100%, 0 100%)"
                : "polygon(0 0, 100% 100%, 0 100%)",
          }}
        />
      </div>
    </>
  );
}
