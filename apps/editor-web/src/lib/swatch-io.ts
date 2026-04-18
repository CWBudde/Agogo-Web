import { clampByte, hsvToRgba, toRgba, type Rgba } from "@/lib/color";

export type SwatchSetRecord = {
  name: string;
  swatches: Rgba[];
};

export async function loadSwatchSetFile(file: File): Promise<SwatchSetRecord> {
  const bytes = new Uint8Array(await file.arrayBuffer());
  const name = file.name.replace(/\.[^.]+$/, "") || "Imported Swatches";
  const lowerName = file.name.toLowerCase();
  if (lowerName.endsWith(".json")) {
    return { name, swatches: parseSwatchSetJSON(new TextDecoder().decode(bytes)) };
  }
  if (lowerName.endsWith(".aco")) {
    return { name, swatches: parseAcoSwatches(bytes) };
  }
  throw new Error("Unsupported swatch file. Use .aco or .json.");
}

export function parseSwatchSetJSON(json: string): Rgba[] {
  const parsed = JSON.parse(json) as unknown;
  const rawSwatches =
    Array.isArray(parsed)
      ? parsed
      : parsed && typeof parsed === "object" && Array.isArray((parsed as { swatches?: unknown[] }).swatches)
        ? (parsed as { swatches: unknown[] }).swatches
        : [];
  const swatches = rawSwatches.flatMap((entry) => {
    const swatch = sanitizeSwatch(entry);
    return swatch ? [swatch] : [];
  });
  if (swatches.length === 0) {
    throw new Error("No swatches found in the imported file.");
  }
  return dedupeSwatches(swatches);
}

export function exportSwatchesAsAco(swatches: Rgba[]) {
  const count = swatches.length;
  const names = swatches.map((_, index) => `Swatch ${index + 1}`);
  const version1Size = 4 + count * 10;
  const version2PayloadSize = swatches.reduce((total, _swatch, index) => {
    return total + 10 + 4 + (names[index].length + 1) * 2;
  }, 0);
  const bytes = new Uint8Array(version1Size + 4 + version2PayloadSize);
  let offset = 0;
  offset = writeUint16(bytes, offset, 1);
  offset = writeUint16(bytes, offset, count);
  for (const swatch of swatches) {
    offset = writeAcoColor(bytes, offset, swatch);
  }
  offset = writeUint16(bytes, offset, 2);
  offset = writeUint16(bytes, offset, count);
  for (let index = 0; index < swatches.length; index++) {
    offset = writeAcoColor(bytes, offset, swatches[index]);
    offset = writeAcoName(bytes, offset, names[index]);
  }
  return bytes;
}

export function parseAcoSwatches(bytes: Uint8Array) {
  let offset = 0;
  let latestColors: Rgba[] = [];
  while (offset + 4 <= bytes.length) {
    const version = readUint16(bytes, offset);
    const count = readUint16(bytes, offset + 2);
    offset += 4;
    const colors: Rgba[] = [];
    for (let index = 0; index < count; index++) {
      if (offset + 10 > bytes.length) {
        throw new Error("ACO file ended unexpectedly.");
      }
      const colorSpace = readUint16(bytes, offset);
      const c1 = readUint16(bytes, offset + 2);
      const c2 = readUint16(bytes, offset + 4);
      const c3 = readUint16(bytes, offset + 6);
      const c4 = readUint16(bytes, offset + 8);
      offset += 10;
      const swatch = decodeAcoColor(colorSpace, c1, c2, c3, c4);
      if (swatch) {
        colors.push(swatch);
      }
      if (version === 2) {
        if (offset + 4 > bytes.length) {
          throw new Error("ACO swatch name data is truncated.");
        }
        const nameLength = readUint32(bytes, offset);
        offset += 4 + nameLength * 2;
        if (offset > bytes.length) {
          throw new Error("ACO swatch name data is truncated.");
        }
      }
    }
    if (colors.length > 0) {
      latestColors = colors;
    }
  }
  if (latestColors.length === 0) {
    throw new Error("No supported swatches found in the ACO file.");
  }
  return dedupeSwatches(latestColors);
}

function sanitizeSwatch(entry: unknown): Rgba | null {
  if (!Array.isArray(entry)) {
    return null;
  }
  return toRgba(entry);
}

function dedupeSwatches(swatches: Rgba[]) {
  const seen = new Set<string>();
  return swatches.filter((swatch) => {
    const key = swatch.join("-");
    if (seen.has(key)) {
      return false;
    }
    seen.add(key);
    return true;
  });
}

function decodeAcoColor(colorSpace: number, c1: number, c2: number, c3: number, _c4: number): Rgba | null {
  switch (colorSpace) {
    case 0:
      return [
        clampByte(c1 / 257),
        clampByte(c2 / 257),
        clampByte(c3 / 257),
        255,
      ];
    case 1:
      return hsvToRgba([(c1 / 65535) * 360, c2 / 65535, c3 / 65535], 255);
    case 8: {
      const gray = clampByte((c1 / 10000) * 255);
      return [gray, gray, gray, 255];
    }
    default:
      return null;
  }
}

function writeAcoColor(bytes: Uint8Array, offset: number, swatch: Rgba) {
  offset = writeUint16(bytes, offset, 0);
  offset = writeUint16(bytes, offset, swatch[0] * 257);
  offset = writeUint16(bytes, offset, swatch[1] * 257);
  offset = writeUint16(bytes, offset, swatch[2] * 257);
  offset = writeUint16(bytes, offset, 0);
  return offset;
}

function writeAcoName(bytes: Uint8Array, offset: number, name: string) {
  const encoded = `${name}\0`;
  offset = writeUint32(bytes, offset, encoded.length);
  for (const character of encoded) {
    offset = writeUint16(bytes, offset, character.charCodeAt(0));
  }
  return offset;
}

function readUint16(bytes: Uint8Array, offset: number) {
  return (bytes[offset] << 8) | bytes[offset + 1];
}

function readUint32(bytes: Uint8Array, offset: number) {
  return (
    (bytes[offset] << 24) |
    (bytes[offset + 1] << 16) |
    (bytes[offset + 2] << 8) |
    bytes[offset + 3]
  ) >>> 0;
}

function writeUint16(bytes: Uint8Array, offset: number, value: number) {
  bytes[offset] = (value >> 8) & 0xff;
  bytes[offset + 1] = value & 0xff;
  return offset + 2;
}

function writeUint32(bytes: Uint8Array, offset: number, value: number) {
  bytes[offset] = (value >> 24) & 0xff;
  bytes[offset + 1] = (value >> 16) & 0xff;
  bytes[offset + 2] = (value >> 8) & 0xff;
  bytes[offset + 3] = value & 0xff;
  return offset + 4;
}
