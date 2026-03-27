import type { SVGProps } from "react";

type IconProps = SVGProps<SVGSVGElement>;

const baseProps = {
  fill: "none",
  stroke: "currentColor",
  strokeLinecap: "round",
  strokeLinejoin: "round",
  strokeWidth: 1.65,
  viewBox: "0 0 24 24",
} satisfies Partial<SVGProps<SVGSVGElement>>;

function IconBase({ children, ...props }: IconProps) {
  return (
    <svg aria-hidden="true" {...baseProps} {...props}>
      {children}
    </svg>
  );
}

export function MoveToolIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="M12 3v18" />
      <path d="m8.5 6.5 3.5-3.5 3.5 3.5" />
      <path d="m8.5 17.5 3.5 3.5 3.5-3.5" />
      <path d="M3 12h18" />
      <path d="m6.5 8.5-3.5 3.5 3.5 3.5" />
      <path d="m17.5 8.5 3.5 3.5-3.5 3.5" />
    </IconBase>
  );
}

export function MarqueeToolIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="M5 8V5h3" />
      <path d="M16 5h3v3" />
      <path d="M19 16v3h-3" />
      <path d="M8 19H5v-3" />
      <path d="M10 5h2" />
      <path d="M14 5h2" />
      <path d="M19 10v2" />
      <path d="M19 14v2" />
      <path d="M10 19h2" />
      <path d="M14 19h2" />
      <path d="M5 10v2" />
      <path d="M5 14v2" />
    </IconBase>
  );
}

export function LassoToolIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="M4.5 12.5c0-4.5 4.2-7.5 8.6-7.5 3.7 0 6.4 1.8 6.4 4.6 0 2.2-1.8 3.9-4.9 4.5l-4.1.8c-1.8.4-2.7 1.2-2.7 2.3 0 1.2 1.2 2 3 2 .9 0 1.7-.2 2.6-.5" />
      <path d="M12.2 17.8c.7-.2 1.3.3 1.3 1 0 .8-.7 1.5-1.6 1.5-.8 0-1.4-.5-1.4-1.3 0-.5.2-.8.6-1.1" />
    </IconBase>
  );
}

export function BrushToolIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="m14 6 4 4" />
      <path d="m11 17 9-9-4-4-9 9-1 5z" />
      <path d="M7 18c-.4 1.7-1.6 2.9-3.5 3" />
    </IconBase>
  );
}

export function EraserToolIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="m9 6 9 9" />
      <path d="m7 8 5-5 8 8-5 5" />
      <path d="m4 11 5 5-2 2H2v-5z" />
      <path d="M11 20h9" />
    </IconBase>
  );
}

export function TypeToolIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="M5 5h14" />
      <path d="M12 5v14" />
      <path d="M8 19h8" />
    </IconBase>
  );
}

export function ShapeToolIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <rect x="4.5" y="4.5" width="7" height="7" />
      <circle cx="16.5" cy="16.5" r="3.5" />
      <path d="M6 18h5" />
    </IconBase>
  );
}

export function HandToolIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <path d="M8 11V6.5a1.5 1.5 0 0 1 3 0V11" />
      <path d="M11 11V5.5a1.5 1.5 0 0 1 3 0V11" />
      <path d="M14 11V7a1.5 1.5 0 0 1 3 0v6.2c0 3.2-1.9 6.1-4.8 7.4L10 21c-1.4.5-2.9-.1-3.7-1.3L4 16v-2.4c0-.9.7-1.6 1.6-1.6.4 0 .8.1 1.1.4L8 13.5" />
    </IconBase>
  );
}

export function ZoomToolIcon(props: IconProps) {
  return (
    <IconBase {...props}>
      <circle cx="10.5" cy="10.5" r="5.5" />
      <path d="M15 15 20 20" />
      <path d="M10.5 8v5" />
      <path d="M8 10.5h5" />
    </IconBase>
  );
}
