import { CommandID, type ImportedBrushLibrary } from "@agogo/proto";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ChangeEvent } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useDocumentIo } from "./use-document-io";

const mocks = vi.hoisted(() => ({
  dispatchCommand: vi.fn(),
  loadAbrLibraries: vi.fn(),
  storeAbrLibrary: vi.fn(),
  setCustomBrushPresets: vi.fn(),
  setBrushPresetStatus: vi.fn(),
  applyBrushPreset: vi.fn(),
}));

vi.mock("@/wasm/context", () => ({
  useEngine: () => ({
    handle: {},
    dispatchCommand: mocks.dispatchCommand,
    exportProject: vi.fn(),
    exportDocument: vi.fn(),
    importProject: vi.fn(),
  }),
  useEngineStore: () => ({ getSnapshot: () => null }),
}));

vi.mock("@/lib/brush-library-storage", () => ({
  loadAbrLibraries: mocks.loadAbrLibraries,
  storeAbrLibrary: mocks.storeAbrLibrary,
}));

vi.mock("@/state/brush-state", () => ({
  useBrushState: () => ({
    customBrushPresets: [],
    setCustomBrushPresets: mocks.setCustomBrushPresets,
    setBrushPresetStatus: mocks.setBrushPresetStatus,
    applyBrushPreset: mocks.applyBrushPreset,
  }),
}));

vi.mock("@/state/color-state", () => ({
  useColorState: () => ({
    setColorSamplerPoints: vi.fn(),
    swatches: [],
    setSwatches: vi.fn(),
    swatchSetName: "",
    setSwatchSetName: vi.fn(),
    setSwatchStatus: vi.fn(),
  }),
}));

vi.mock("@/state/shape-state", () => ({
  useShapeState: () => ({
    customShapePresets: [],
    setCustomShapePresets: vi.fn(),
    setShapePresetId: vi.fn(),
    setShapePresetStatus: vi.fn(),
    setShapeSubTool: vi.fn(),
  }),
}));

vi.mock("@/state/view-state", () => ({
  useViewState: () => ({ setActiveAuxPanel: vi.fn() }),
}));

const importedLibrary: ImportedBrushLibrary = {
  libraryId: "library-1",
  name: "Studio Brushes",
  presets: [
    {
      id: "engine-ink",
      name: "Engine Ink",
      tipResourceId: "tip-7",
      thumbnailRGBA: "AAECAw==",
      size: 48,
      hardness: 0.65,
      spacing: 0.18,
      angle: 32,
      roundness: 0.72,
      sizeJitter: 0.25,
      opacityJitter: 0.15,
      flowJitter: 0.05,
      controlSource: "pressure",
      fadeDabs: 240,
      warnings: ["Unsupported texture behavior"],
    },
  ],
};
const draft = {
  width: 100,
  height: 100,
  name: "Untitled",
  resolution: 72,
  colorMode: "rgb" as const,
  bitDepth: 8 as const,
  background: "transparent" as const,
};

beforeEach(() => {
  localStorage.clear();
  Object.values(mocks).forEach((mock) => {
    mock.mockReset();
  });
  mocks.loadAbrLibraries.mockResolvedValue([]);
  mocks.storeAbrLibrary.mockResolvedValue(undefined);
  mocks.dispatchCommand.mockReturnValue({ importedBrushLibrary: importedLibrary });
});

describe("useDocumentIo ABR integration", () => {
  it("sends ABR bytes to the engine, maps its registered tip, and persists the library", async () => {
    const { result } = renderHook(() =>
      useDocumentIo({
        draft,
        setDraft: vi.fn(),
      }),
    );
    const input = { value: "selected", files: [] as unknown as FileList };
    const file = {
      name: "studio.abr",
      arrayBuffer: async () => Uint8Array.from([1, 2, 3]).buffer,
    } as File;
    Object.defineProperty(input, "files", { value: [file] });

    await act(async () => {
      await result.current.handleBrushPresetInputChange({
        target: input,
      } as unknown as ChangeEvent<HTMLInputElement>);
    });

    expect(input.value).toBe("");
    expect(mocks.dispatchCommand).toHaveBeenCalledWith(CommandID.ImportAbrBrushLibrary, {
      data: "AQID",
      fileName: "studio.abr",
    });
    const mappedPreset = expect.objectContaining({
      id: "engine-ink",
      name: "Engine Ink",
      tipShape: "round",
      tipResourceId: "tip-7",
      thumbnailRGBA: "AAECAw==",
      size: 48,
      spacing: 0.18,
      hardness: 0.65,
      angle: 32,
      roundness: 0.72,
      sizeJitter: 0.25,
      opacityJitter: 0.15,
      flowJitter: 0.05,
      controlSource: "pressure",
      fadeDabs: 240,
    });
    expect(mocks.setCustomBrushPresets).toHaveBeenCalledWith([mappedPreset]);
    expect(mocks.applyBrushPreset).toHaveBeenCalledWith(mappedPreset);
    expect(mocks.storeAbrLibrary).toHaveBeenCalledWith({
      libraryId: "library-1",
      fileName: "studio.abr",
      data: "AQID",
      imported: importedLibrary,
    });
    expect(mocks.setBrushPresetStatus).toHaveBeenCalledWith(
      "Imported 1 brush preset from studio.abr.",
    );
  });

  it("restores persisted ABR bytes through the engine registry on startup", async () => {
    mocks.loadAbrLibraries.mockResolvedValue([
      {
        libraryId: "library-1",
        fileName: "studio.abr",
        data: "AQID",
        imported: importedLibrary,
      },
    ]);
    renderHook(() =>
      useDocumentIo({
        draft,
        setDraft: vi.fn(),
      }),
    );

    await waitFor(() =>
      expect(mocks.dispatchCommand).toHaveBeenCalledWith(CommandID.ImportAbrBrushLibrary, {
        data: "AQID",
        fileName: "studio.abr",
      }),
    );
    expect(mocks.setCustomBrushPresets).toHaveBeenCalledWith([
      expect.objectContaining({ id: "engine-ink", tipResourceId: "tip-7" }),
    ]);
  });

  it("reports a missing engine import response without persisting partial data", async () => {
    mocks.dispatchCommand.mockReturnValue({});
    const { result } = renderHook(() =>
      useDocumentIo({
        draft,
        setDraft: vi.fn(),
      }),
    );
    const input = { value: "selected", files: [] as unknown as FileList };
    const file = {
      name: "broken.abr",
      arrayBuffer: async () => Uint8Array.from([1]).buffer,
    } as File;
    Object.defineProperty(input, "files", { value: [file] });

    await act(async () => {
      await result.current.handleBrushPresetInputChange({
        target: input,
      } as unknown as ChangeEvent<HTMLInputElement>);
    });

    expect(mocks.storeAbrLibrary).not.toHaveBeenCalled();
    expect(mocks.setBrushPresetStatus).toHaveBeenCalledWith(
      "The engine did not return an imported ABR library.",
    );
  });
});
