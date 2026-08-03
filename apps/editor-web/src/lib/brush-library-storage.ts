import type { ImportedBrushLibrary } from "@agogo/proto";

const DATABASE_NAME = "agogo-brush-libraries";
const DATABASE_VERSION = 1;
const STORE_NAME = "abr-libraries";

export interface StoredAbrLibrary {
  libraryId: string;
  fileName: string;
  data: string;
  imported: ImportedBrushLibrary;
}

function openDatabase(): Promise<IDBDatabase | null> {
  if (typeof indexedDB === "undefined") {
    return Promise.resolve(null);
  }
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DATABASE_NAME, DATABASE_VERSION);
    request.onupgradeneeded = () => {
      const database = request.result;
      if (!database.objectStoreNames.contains(STORE_NAME)) {
        database.createObjectStore(STORE_NAME, { keyPath: "libraryId" });
      }
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () =>
      reject(request.error ?? new Error("Could not open brush library storage."));
  });
}

export async function storeAbrLibrary(library: StoredAbrLibrary): Promise<void> {
  const database = await openDatabase();
  if (!database) {
    return;
  }
  await new Promise<void>((resolve, reject) => {
    const transaction = database.transaction(STORE_NAME, "readwrite");
    transaction.objectStore(STORE_NAME).put(library);
    transaction.oncomplete = () => resolve();
    transaction.onerror = () =>
      reject(transaction.error ?? new Error("Could not save the imported brush library."));
  });
  database.close();
}

export async function loadAbrLibraries(): Promise<StoredAbrLibrary[]> {
  const database = await openDatabase();
  if (!database) {
    return [];
  }
  const libraries = await new Promise<StoredAbrLibrary[]>((resolve, reject) => {
    const transaction = database.transaction(STORE_NAME, "readonly");
    const request = transaction.objectStore(STORE_NAME).getAll();
    request.onsuccess = () => resolve(request.result as StoredAbrLibrary[]);
    request.onerror = () => reject(request.error ?? new Error("Could not load brush libraries."));
  });
  database.close();
  return libraries;
}
