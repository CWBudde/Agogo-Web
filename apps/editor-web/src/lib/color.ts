export type Rgba = readonly [number, number, number, number];
export type Rgb = readonly [number, number, number];
export type Hsv = readonly [number, number, number];

export function clampByte(value: number): number {
  if (Number.isNaN(value)) {
    return 0;
  }
  return Math.max(0, Math.min(255, Math.round(value)));
}

export function clampUnit(value: number): number {
  if (Number.isNaN(value)) {
    return 0;
  }
  return Math.max(0, Math.min(1, value));
}

export function rgbaToHex(color: Rgba): string {
  return `#${color
    .slice(0, 3)
    .map((component) => clampByte(component).toString(16).padStart(2, "0"))
    .join("")}`;
}

export function hexToRgba(hex: string): Rgba | null {
  const normalized = hex.trim().replace(/^#/, "");
  if (normalized.length !== 3 && normalized.length !== 6) {
    return null;
  }

  const expanded =
    normalized.length === 3
      ? normalized
          .split("")
          .map((component) => component.repeat(2))
          .join("")
      : normalized;

  if (!/^[0-9a-fA-F]{6}$/.test(expanded)) {
    return null;
  }

  const r = Number.parseInt(expanded.slice(0, 2), 16);
  const g = Number.parseInt(expanded.slice(2, 4), 16);
  const b = Number.parseInt(expanded.slice(4, 6), 16);
  return [r, g, b, 255];
}

export function rgbToHsv([r, g, b]: Rgb): Hsv {
  const normalizedR = r / 255;
  const normalizedG = g / 255;
  const normalizedB = b / 255;
  const max = Math.max(normalizedR, normalizedG, normalizedB);
  const min = Math.min(normalizedR, normalizedG, normalizedB);
  const delta = max - min;

  let hue = 0;
  if (delta !== 0) {
    if (max === normalizedR) {
      hue = ((normalizedG - normalizedB) / delta) % 6;
    } else if (max === normalizedG) {
      hue = (normalizedB - normalizedR) / delta + 2;
    } else {
      hue = (normalizedR - normalizedG) / delta + 4;
    }
    hue *= 60;
    if (hue < 0) {
      hue += 360;
    }
  }

  const saturation = max === 0 ? 0 : delta / max;
  return [hue, saturation, max];
}

export function hsvToRgb([hue, saturation, value]: Hsv): Rgb {
  const normalizedHue = ((hue % 360) + 360) % 360;
  const chroma = value * saturation;
  const x = chroma * (1 - Math.abs(((normalizedHue / 60) % 2) - 1));
  const match = value - chroma;

  let normalizedR = 0;
  let normalizedG = 0;
  let normalizedB = 0;

  if (normalizedHue < 60) {
    normalizedR = chroma;
    normalizedG = x;
  } else if (normalizedHue < 120) {
    normalizedR = x;
    normalizedG = chroma;
  } else if (normalizedHue < 180) {
    normalizedG = chroma;
    normalizedB = x;
  } else if (normalizedHue < 240) {
    normalizedG = x;
    normalizedB = chroma;
  } else if (normalizedHue < 300) {
    normalizedR = x;
    normalizedB = chroma;
  } else {
    normalizedR = chroma;
    normalizedB = x;
  }

  return [
    clampByte((normalizedR + match) * 255),
    clampByte((normalizedG + match) * 255),
    clampByte((normalizedB + match) * 255),
  ];
}

export function rgbaToHsv(color: Rgba): Hsv {
  return rgbToHsv([color[0], color[1], color[2]]);
}

export function hsvToRgba(hsv: Hsv, alpha = 255): Rgba {
  const [r, g, b] = hsvToRgb(hsv);
  return [r, g, b, clampByte(alpha)];
}

export function isWebSafeColor(color: Rgba): boolean {
  return color.slice(0, 3).every((component) => clampByte(component) % 51 === 0);
}

export function snapToWebSafeColor(color: Rgba): Rgba {
  return color.map((component, index) => {
    if (index === 3) {
      return clampByte(component);
    }
    const snapped = Math.round(clampByte(component) / 51) * 51;
    return Math.max(0, Math.min(255, snapped));
  }) as Rgba;
}

export function rgbaToCss(color: Rgba): string {
  return `rgba(${color.map((component) => clampByte(component)).join(", ")})`;
}

export function formatPercent(value: number): string {
  return `${Math.round(clampUnit(value) * 100)}%`;
}

export function colorEquals(left: Rgba, right: Rgba): boolean {
  return left.every((component, index) => clampByte(component) === clampByte(right[index]));
}
