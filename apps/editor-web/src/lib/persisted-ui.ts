import type { GradientStopCommand } from "@agogo/proto";
import type { BrushPreset } from "@/components/brush-color-panels";
import { parseBrushPresetJSON } from "@/lib/brush-preset-io";
import { type Rgba, toMutableRgba } from "@/lib/color";
import { parseShapePresetJSON } from "@/lib/shape-preset-io";
import type { ShapePreset } from "@/lib/shape-presets";

export const RECENT_COLORS_KEY = "agogo:recent-colors";
export const CUSTOM_BRUSH_PRESETS_KEY = "agogo:custom-brush-presets";
export const CUSTOM_SHAPE_PRESETS_KEY = "agogo:custom-shape-presets";
export const CUSTOM_SWATCHES_KEY = "agogo:custom-swatches";
export const CUSTOM_SWATCHES_NAME_KEY = "agogo:custom-swatches-name";
export const GRADIENT_STOPS_KEY = "agogo:gradient-stops";

export function loadColorList(key: string, fallback: Rgba[]): Rgba[] {
  if (typeof window === "undefined") {
    return fallback;
  }
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) {
      return fallback;
    }
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) {
      return fallback;
    }
    return parsed
      .map((entry) => {
        if (!Array.isArray(entry) || entry.length < 4) {
          return null;
        }
        return [Number(entry[0]), Number(entry[1]), Number(entry[2]), Number(entry[3])] as Rgba;
      })
      .filter((entry): entry is Rgba => entry !== null);
  } catch {
    return fallback;
  }
}

export function loadBrushPresetList(key: string, fallback: BrushPreset[] = []): BrushPreset[] {
  if (typeof window === "undefined") {
    return fallback;
  }
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) {
      return fallback;
    }
    return parseBrushPresetJSON(raw, "Imported Brushes");
  } catch {
    return fallback;
  }
}

export function loadShapePresetList(key: string, fallback: ShapePreset[] = []): ShapePreset[] {
  if (typeof window === "undefined") {
    return fallback;
  }
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) {
      return fallback;
    }
    return parseShapePresetJSON(raw, "Imported Shapes");
  } catch {
    return fallback;
  }
}

export function loadStoredName(key: string, fallback: string): string {
  if (typeof window === "undefined") {
    return fallback;
  }
  try {
    return window.localStorage.getItem(key) ?? fallback;
  } catch {
    return fallback;
  }
}

export function loadGradientStops(
  key: string,
  fallback: GradientStopCommand[],
): GradientStopCommand[] {
  if (typeof window === "undefined") {
    return fallback;
  }
  try {
    const raw = window.localStorage.getItem(key);
    if (!raw) {
      return fallback;
    }
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) {
      return fallback;
    }
    return parsed
      .map((entry) => {
        if (typeof entry !== "object" || entry === null) {
          return null;
        }
        const candidate = entry as { position?: unknown; color?: unknown };
        if (
          typeof candidate.position !== "number" ||
          !Array.isArray(candidate.color) ||
          candidate.color.length < 4
        ) {
          return null;
        }
        return {
          position: candidate.position,
          color: toMutableRgba(candidate.color),
        } satisfies GradientStopCommand;
      })
      .filter((entry): entry is GradientStopCommand => entry !== null);
  } catch {
    return fallback;
  }
}
