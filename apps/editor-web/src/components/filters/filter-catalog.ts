import type { FilterParams } from "@agogo/proto";

/**
 * Frontend mirror of the engine's filter registry
 * (packages/engine-wasm/internal/engine/filters_builtin_registry.go). The
 * engine exposes no runtime discovery command, so the catalog — ids, display
 * names, categories, and the parameter schema each dialog renders — is carried
 * statically here. Field `name`s are the snake_case JSON keys the Go filter
 * functions decode; keep them in sync with the `*Params` structs.
 */

export type FilterCategory =
  | "blur"
  | "sharpen"
  | "noise"
  | "distort"
  | "stylize"
  | "render"
  | "other";

export interface FilterSelectOption {
  value: string;
  label: string;
}

export type FilterParamField =
  | {
      kind: "number";
      name: string;
      label: string;
      min: number;
      max: number;
      step?: number;
      default: number;
      unit?: string;
    }
  | {
      kind: "select";
      name: string;
      label: string;
      options: FilterSelectOption[];
      default: string;
    }
  | {
      kind: "checkbox";
      name: string;
      label: string;
      default: boolean;
    };

export interface FilterDefinition {
  id: string;
  name: string;
  category: FilterCategory;
  hasDialog: boolean;
  fields: FilterParamField[];
}

/** Menu order for filter category submenus. */
export const FILTER_CATEGORY_ORDER: FilterCategory[] = [
  "blur",
  "sharpen",
  "noise",
  "distort",
  "stylize",
  "render",
  "other",
];

export const FILTER_CATEGORY_LABELS: Record<FilterCategory, string> = {
  blur: "Blur",
  sharpen: "Sharpen",
  noise: "Noise",
  distort: "Distort",
  stylize: "Stylize",
  render: "Render",
  other: "Other",
};

const num = (
  name: string,
  label: string,
  min: number,
  max: number,
  def: number,
  extra?: { step?: number; unit?: string },
): FilterParamField => ({
  kind: "number",
  name,
  label,
  min,
  max,
  default: def,
  step: extra?.step,
  unit: extra?.unit,
});

const select = (
  name: string,
  label: string,
  options: FilterSelectOption[],
  def: string,
): FilterParamField => ({ kind: "select", name, label, options, default: def });

const check = (name: string, label: string, def: boolean): FilterParamField => ({
  kind: "checkbox",
  name,
  label,
  default: def,
});

export const FILTER_CATALOG: FilterDefinition[] = [
  { id: "invert", name: "Invert", category: "other", hasDialog: false, fields: [] },
  {
    id: "gaussian-blur",
    name: "Gaussian Blur",
    category: "blur",
    hasDialog: true,
    fields: [num("radius", "Radius", 1, 250, 5, { unit: "px" })],
  },
  {
    id: "box-blur",
    name: "Box Blur",
    category: "blur",
    hasDialog: true,
    fields: [num("radius", "Radius", 1, 250, 5, { unit: "px" })],
  },
  {
    id: "motion-blur",
    name: "Motion Blur",
    category: "blur",
    hasDialog: true,
    fields: [
      num("angle", "Angle", 0, 360, 0, { unit: "°" }),
      num("distance", "Distance", 1, 500, 10, { unit: "px" }),
    ],
  },
  {
    id: "radial-blur",
    name: "Radial Blur",
    category: "blur",
    hasDialog: true,
    fields: [
      select(
        "type",
        "Method",
        [
          { value: "spin", label: "Spin" },
          { value: "zoom", label: "Zoom" },
        ],
        "spin",
      ),
      num("amount", "Amount", 1, 100, 10),
      select(
        "quality",
        "Quality",
        [
          { value: "1", label: "Draft" },
          { value: "2", label: "Good" },
          { value: "3", label: "Best" },
        ],
        "3",
      ),
    ],
  },
  {
    id: "surface-blur",
    name: "Surface Blur",
    category: "blur",
    hasDialog: true,
    fields: [
      num("radius", "Radius", 1, 100, 5, { unit: "px" }),
      num("threshold", "Threshold", 0, 255, 15),
    ],
  },
  {
    id: "sharpen",
    name: "Sharpen",
    category: "sharpen",
    hasDialog: false,
    fields: [],
  },
  {
    id: "sharpen-more",
    name: "Sharpen More",
    category: "sharpen",
    hasDialog: false,
    fields: [],
  },
  {
    id: "unsharp-mask",
    name: "Unsharp Mask",
    category: "sharpen",
    hasDialog: true,
    fields: [
      num("amount", "Amount", 1, 500, 50, { unit: "%" }),
      num("radius", "Radius", 1, 250, 1, { unit: "px" }),
      num("threshold", "Threshold", 0, 255, 0),
    ],
  },
  {
    id: "smart-sharpen",
    name: "Smart Sharpen",
    category: "sharpen",
    hasDialog: true,
    fields: [
      num("amount", "Amount", 1, 500, 100, { unit: "%" }),
      num("radius", "Radius", 1, 64, 3, { unit: "px" }),
      select(
        "remove",
        "Remove",
        [
          { value: "gaussian", label: "Gaussian Blur" },
          { value: "lens", label: "Lens Blur" },
          { value: "motion", label: "Motion Blur" },
        ],
        "lens",
      ),
      num("angle", "Angle", 0, 360, 0, { unit: "°" }),
      num("shadow_fade", "Fade Shadows", 0, 100, 0, { unit: "%" }),
      num("highlight_fade", "Fade Highlights", 0, 100, 0, { unit: "%" }),
    ],
  },
  {
    id: "add-noise",
    name: "Add Noise",
    category: "noise",
    hasDialog: true,
    fields: [
      num("amount", "Amount", 0, 400, 25, { unit: "%" }),
      select(
        "distribution",
        "Distribution",
        [
          { value: "uniform", label: "Uniform" },
          { value: "gaussian", label: "Gaussian" },
        ],
        "gaussian",
      ),
      check("monochromatic", "Monochromatic", false),
    ],
  },
  {
    id: "median",
    name: "Median",
    category: "noise",
    hasDialog: true,
    fields: [num("radius", "Radius", 1, 100, 2, { unit: "px" })],
  },
  {
    id: "despeckle",
    name: "Despeckle",
    category: "noise",
    hasDialog: false,
    fields: [],
  },
  {
    id: "reduce-noise",
    name: "Reduce Noise",
    category: "noise",
    hasDialog: true,
    fields: [
      num("strength", "Strength", 0, 10, 5),
      num("preserve_details", "Preserve Details", 0, 100, 60, { unit: "%" }),
      num("reduce_color_noise", "Reduce Color Noise", 0, 100, 45, { unit: "%" }),
      num("sharpen_details", "Sharpen Details", 0, 100, 25, { unit: "%" }),
    ],
  },
  {
    id: "ripple",
    name: "Ripple",
    category: "distort",
    hasDialog: true,
    fields: [
      num("amount", "Amount", -999, 999, 100),
      select(
        "size",
        "Size",
        [
          { value: "small", label: "Small" },
          { value: "medium", label: "Medium" },
          { value: "large", label: "Large" },
        ],
        "medium",
      ),
    ],
  },
  {
    id: "twirl",
    name: "Twirl",
    category: "distort",
    hasDialog: true,
    fields: [num("angle", "Angle", -999, 999, 50, { unit: "°" })],
  },
  {
    id: "offset",
    name: "Offset",
    category: "distort",
    hasDialog: true,
    fields: [
      num("horizontal", "Horizontal", -2000, 2000, 0, { unit: "px" }),
      num("vertical", "Vertical", -2000, 2000, 0, { unit: "px" }),
      select(
        "wrap",
        "Undefined Areas",
        [
          { value: "wrap", label: "Wrap Around" },
          { value: "repeat", label: "Repeat Edge Pixels" },
        ],
        "wrap",
      ),
    ],
  },
  {
    id: "polar-coordinates",
    name: "Polar Coordinates",
    category: "distort",
    hasDialog: true,
    fields: [
      select(
        "mode",
        "Conversion",
        [
          { value: "rectangular-to-polar", label: "Rectangular to Polar" },
          { value: "polar-to-rectangular", label: "Polar to Rectangular" },
        ],
        "rectangular-to-polar",
      ),
    ],
  },
  {
    id: "lens-correction",
    name: "Lens Correction",
    category: "distort",
    hasDialog: true,
    fields: [
      num("distortion", "Distortion", -100, 100, 0),
      num("chromatic_aberration", "Chromatic Aberration", 0, 100, 0),
      num("vignette", "Vignette", -100, 100, 0),
      num("perspective_vertical", "Vertical Perspective", -100, 100, 0),
      num("perspective_horizontal", "Horizontal Perspective", -100, 100, 0),
    ],
  },
  {
    id: "emboss",
    name: "Emboss",
    category: "stylize",
    hasDialog: true,
    fields: [
      num("angle", "Angle", 0, 360, 135, { unit: "°" }),
      num("height", "Height", 1, 10, 3, { unit: "px" }),
      num("amount", "Amount", 1, 500, 100, { unit: "%" }),
    ],
  },
  {
    id: "solarize",
    name: "Solarize",
    category: "stylize",
    hasDialog: false,
    fields: [],
  },
  {
    id: "find-edges",
    name: "Find Edges",
    category: "stylize",
    hasDialog: false,
    fields: [],
  },
  {
    id: "brightness-contrast",
    name: "Brightness/Contrast",
    category: "other",
    hasDialog: true,
    fields: [
      num("brightness", "Brightness", -150, 150, 0),
      num("contrast", "Contrast", -100, 100, 0),
    ],
  },
  {
    id: "high-pass",
    name: "High Pass",
    category: "other",
    hasDialog: true,
    fields: [num("radius", "Radius", 1, 250, 10, { unit: "px" })],
  },
  {
    id: "minimum",
    name: "Minimum",
    category: "other",
    hasDialog: true,
    fields: [num("radius", "Radius", 1, 100, 1, { unit: "px" })],
  },
  {
    id: "maximum",
    name: "Maximum",
    category: "other",
    hasDialog: true,
    fields: [num("radius", "Radius", 1, 100, 1, { unit: "px" })],
  },
];

const catalogById = new Map(FILTER_CATALOG.map((def) => [def.id, def]));

/** Case-insensitive lookup matching the engine's normalizeFilterID. */
export function getFilterDefinition(id: string): FilterDefinition | undefined {
  return catalogById.get(id.trim().toLowerCase());
}

/** Build the default parameter object from a filter definition's fields. */
export function defaultFilterParams(def: FilterDefinition): FilterParams {
  const params: FilterParams = {};
  for (const field of def.fields) {
    params[field.name] = field.default;
  }
  return params;
}

export interface FilterCategoryGroup {
  category: FilterCategory;
  label: string;
  filters: FilterDefinition[];
}

/** Group the catalog by category in menu order, dropping empty categories. */
export function filtersByCategory(): FilterCategoryGroup[] {
  return FILTER_CATEGORY_ORDER.map((category) => ({
    category,
    label: FILTER_CATEGORY_LABELS[category],
    filters: FILTER_CATALOG.filter((def) => def.category === category),
  })).filter((group) => group.filters.length > 0);
}
