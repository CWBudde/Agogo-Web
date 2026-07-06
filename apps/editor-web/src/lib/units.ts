export type DocumentUnit = "px" | "in" | "cm" | "mm";

export const unitSteps: Record<DocumentUnit, number> = {
  px: 1,
  in: 0.01,
  cm: 0.1,
  mm: 1,
};

export function pixelsToUnit(pixels: number, resolution: number, unit: DocumentUnit) {
  switch (unit) {
    case "in":
      return pixels / resolution;
    case "cm":
      return (pixels / resolution) * 2.54;
    case "mm":
      return (pixels / resolution) * 25.4;
    default:
      return pixels;
  }
}

export function unitToPixels(value: number, resolution: number, unit: DocumentUnit) {
  switch (unit) {
    case "in":
      return value * resolution;
    case "cm":
      return (value / 2.54) * resolution;
    case "mm":
      return (value / 25.4) * resolution;
    default:
      return value;
  }
}

export function formatDimension(value: number, unit: DocumentUnit) {
  if (unit === "px" || unit === "mm") {
    return Math.round(value).toString();
  }
  return value.toFixed(2);
}
