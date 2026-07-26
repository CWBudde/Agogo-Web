import { describe, expect, it } from "vitest";
import {
  defaultFilterParams,
  FILTER_CATALOG,
  FILTER_CATEGORY_ORDER,
  filtersByCategory,
  getFilterDefinition,
} from "./filter-catalog";

function requireFilter(id: string) {
  const def = getFilterDefinition(id);
  if (!def) {
    throw new Error(`filter ${id} not in catalog`);
  }
  return def;
}

describe("filter catalog", () => {
  it("registers every engine builtin filter", () => {
    // Mirrors registerBuiltinFilters() in the Go engine (26 filters).
    expect(FILTER_CATALOG).toHaveLength(26);
    const ids = FILTER_CATALOG.map((f) => f.id);
    expect(new Set(ids).size).toBe(ids.length); // ids are unique
    expect(ids).toContain("gaussian-blur");
    expect(ids).toContain("lens-correction");
  });

  it("looks up a filter by id, case-insensitively", () => {
    const def = getFilterDefinition("Gaussian-Blur");
    expect(def?.id).toBe("gaussian-blur");
    expect(def?.name).toBe("Gaussian Blur");
    expect(def?.category).toBe("blur");
    expect(def?.hasDialog).toBe(true);
  });

  it("returns undefined for an unknown filter id", () => {
    expect(getFilterDefinition("does-not-exist")).toBeUndefined();
  });

  it("marks parameterless filters as dialog-less with no fields", () => {
    const invert = getFilterDefinition("invert");
    expect(invert?.hasDialog).toBe(false);
    expect(invert?.fields).toEqual([]);
  });

  it("builds default params from each dialog filter's fields", () => {
    expect(defaultFilterParams(requireFilter("gaussian-blur"))).toEqual({ radius: 5 });
    expect(defaultFilterParams(requireFilter("add-noise"))).toEqual({
      amount: 25,
      distribution: "gaussian",
      monochromatic: false,
    });
  });

  it("gives every dialog filter at least one field and every field a matching default", () => {
    for (const def of FILTER_CATALOG) {
      if (def.hasDialog) {
        expect(def.fields.length).toBeGreaterThan(0);
      }
      const params = defaultFilterParams(def);
      for (const field of def.fields) {
        expect(params[field.name]).toBe(field.default);
      }
    }
  });

  it("groups filters by category in menu order, omitting empty categories", () => {
    const groups = filtersByCategory();
    const categories = groups.map((g) => g.category);
    // Every group is non-empty and appears in the canonical order.
    for (const g of groups) {
      expect(g.filters.length).toBeGreaterThan(0);
    }
    const orderIndex = categories.map((c) => FILTER_CATEGORY_ORDER.indexOf(c));
    expect(orderIndex).toEqual([...orderIndex].sort((a, b) => a - b));
    expect(categories[0]).toBe("blur");
    // Total filters across groups equals the catalog size.
    expect(groups.reduce((n, g) => n + g.filters.length, 0)).toBe(FILTER_CATALOG.length);
  });
});
