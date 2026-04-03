import type { PathOverlay } from "@agogo/proto";

interface PathOverlayRendererProps {
  overlay: PathOverlay;
}

export function PathOverlayRenderer({ overlay }: PathOverlayRendererProps) {
  return (
    <svg
      className="pointer-events-none absolute inset-0"
      style={{ width: "100%", height: "100%" }}
    >
      <title>Path overlay</title>
      {/* Path segments — blue lines */}
      {overlay.segments?.map((seg, i) => (
        <polyline
          key={`seg-${i.toString()}`}
          points={seg.points.map((p) => `${p.x},${p.y}`).join(" ")}
          fill="none"
          stroke="#00a8ff"
          strokeWidth={1}
        />
      ))}
      {/* Handle lines — gray lines from anchor to handle */}
      {overlay.handleLines?.map((line, i) => (
        <line
          key={`hl-${i.toString()}`}
          x1={line.x1}
          y1={line.y1}
          x2={line.x2}
          y2={line.y2}
          stroke="#888"
          strokeWidth={1}
        />
      ))}
      {/* Rubber band preview — dashed blue */}
      {overlay.rubberBand && (
        <polyline
          points={overlay.rubberBand.points
            .map((p) => `${p.x},${p.y}`)
            .join(" ")}
          fill="none"
          stroke="#00a8ff"
          strokeWidth={1}
          strokeDasharray="4 4"
        />
      )}
      {/* Anchor points — small squares */}
      {overlay.anchors?.map((a, i) => (
        <rect
          key={`a-${i.toString()}`}
          x={a.x - 3}
          y={a.y - 3}
          width={6}
          height={6}
          fill={a.selected ? "#00a8ff" : "white"}
          stroke="#333"
          strokeWidth={1}
        />
      ))}
    </svg>
  );
}
