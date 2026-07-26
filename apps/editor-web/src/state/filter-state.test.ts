import { describe, expect, it } from "vitest";
import { filterReducer, initialFilterState } from "./filter-state";

describe("filterReducer", () => {
  it("opens a filter dialog and closes any open fade dialog", () => {
    const state = filterReducer(
      { ...initialFilterState, fadeOpen: true },
      { type: "open-filter", id: "gaussian-blur" },
    );
    expect(state.activeFilterId).toBe("gaussian-blur");
    expect(state.fadeOpen).toBe(false);
  });

  it("closes the active filter dialog", () => {
    const open = filterReducer(initialFilterState, { type: "open-filter", id: "twirl" });
    expect(filterReducer(open, { type: "close-filter" }).activeFilterId).toBeNull();
  });

  it("opening the fade dialog closes any active filter dialog", () => {
    const open = filterReducer(initialFilterState, { type: "open-filter", id: "twirl" });
    const fade = filterReducer(open, { type: "open-fade" });
    expect(fade.fadeOpen).toBe(true);
    expect(fade.activeFilterId).toBeNull();
  });

  it("records the last-applied filter and makes fade available", () => {
    const state = filterReducer(initialFilterState, {
      type: "applied",
      id: "gaussian-blur",
      name: "Gaussian Blur",
    });
    expect(state.lastFilter).toEqual({ id: "gaussian-blur", name: "Gaussian Blur" });
    expect(state.canFade).toBe(true);
  });

  it("fade is a one-shot: it clears fade availability but keeps the last filter", () => {
    const applied = filterReducer(initialFilterState, {
      type: "applied",
      id: "twirl",
      name: "Twirl",
    });
    const faded = filterReducer(applied, { type: "faded" });
    expect(faded.canFade).toBe(false);
    expect(faded.lastFilter).toEqual({ id: "twirl", name: "Twirl" });
  });
});
