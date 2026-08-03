import type { ImportedBrushLibrary } from "@agogo/proto";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadAbrLibraries, type StoredAbrLibrary, storeAbrLibrary } from "./brush-library-storage";

const imported: ImportedBrushLibrary = {
  libraryId: "library-1",
  name: "Studio Brushes",
  presets: [{ id: "brush-1", name: "Ink" }],
};

function createIndexedDbMock(initial: StoredAbrLibrary[] = []) {
  const records = new Map(initial.map((library) => [library.libraryId, library]));
  const close = vi.fn();
  const createObjectStore = vi.fn();
  const database = {
    objectStoreNames: { contains: vi.fn(() => false) },
    createObjectStore,
    close,
    transaction: vi.fn((_name: string, _mode: IDBTransactionMode) => {
      const transaction = {
        error: null,
        oncomplete: null as (() => void) | null,
        onerror: null as (() => void) | null,
        objectStore: () => ({
          put: (library: StoredAbrLibrary) => {
            records.set(library.libraryId, library);
            queueMicrotask(() => transaction.oncomplete?.());
          },
          getAll: () => {
            const request = {
              result: [...records.values()],
              error: null,
              onsuccess: null as (() => void) | null,
              onerror: null as (() => void) | null,
            };
            queueMicrotask(() => request.onsuccess?.());
            return request;
          },
        }),
      };
      return transaction;
    }),
  };
  const open = vi.fn(() => {
    const request = {
      result: database,
      error: null,
      onupgradeneeded: null as (() => void) | null,
      onsuccess: null as (() => void) | null,
      onerror: null as (() => void) | null,
    };
    queueMicrotask(() => {
      request.onupgradeneeded?.();
      request.onsuccess?.();
    });
    return request;
  });
  return { indexedDB: { open }, records, database, createObjectStore, close };
}

describe("brush library storage", () => {
  beforeEach(() => {
    vi.unstubAllGlobals();
  });

  it("stores and loads the original ABR bytes with imported engine metadata", async () => {
    const mock = createIndexedDbMock();
    vi.stubGlobal("indexedDB", mock.indexedDB);
    const library: StoredAbrLibrary = {
      libraryId: imported.libraryId,
      fileName: "studio.abr",
      data: "AQID",
      imported,
    };

    await storeAbrLibrary(library);
    await expect(loadAbrLibraries()).resolves.toEqual([library]);
    expect(mock.records.get("library-1")).toEqual(library);
    expect(mock.createObjectStore).toHaveBeenCalledWith("abr-libraries", {
      keyPath: "libraryId",
    });
    expect(mock.close).toHaveBeenCalledTimes(2);
  });

  it("degrades safely when IndexedDB is unavailable", async () => {
    vi.stubGlobal("indexedDB", undefined);
    await expect(
      storeAbrLibrary({
        libraryId: imported.libraryId,
        fileName: "studio.abr",
        data: "AQID",
        imported,
      }),
    ).resolves.toBeUndefined();
    await expect(loadAbrLibraries()).resolves.toEqual([]);
  });
});
