import { describe, expect, it } from "vitest";
import { parseCshShapePresets, parseShapePresetJSON } from "@/lib/shape-preset-io";

describe("shape preset import", () => {
  it("parses compound shape presets from JSON", () => {
    const presets = parseShapePresetJSON(
      JSON.stringify({
        presets: [
          {
            name: "Compound",
            subpaths: [
              {
                closed: true,
                points: [
                  { x: 0, y: 0 },
                  { x: 1, y: 0 },
                  { x: 1, y: 1 },
                ],
              },
              {
                closed: false,
                points: [
                  { x: 0.25, y: 0.5 },
                  { x: 0.75, y: 0.5 },
                ],
              },
            ],
          },
        ],
      }),
      "Shapes",
    );

    expect(presets).toHaveLength(1);
    expect(presets[0].subpaths).toHaveLength(2);
    expect(presets[0].category).toBe("imported");
  });

  it("parses 8BIM path resources from CSH bytes", () => {
    const bytes = concatBytes([
      new Uint8Array([0, 1, 0, 0]),
      makeResourceBlock("Diamond", 2000, makePathResourceData()),
    ]);

    const presets = parseCshShapePresets(bytes, "Imported Shapes");

    expect(presets).toHaveLength(1);
    expect(presets[0].name).toBe("Diamond");
    expect(presets[0].subpaths).toHaveLength(2);
    expect(presets[0].subpaths?.[0].closed).toBe(true);
    expect(presets[0].subpaths?.[1].closed).toBe(false);
    expect(presets[0].subpaths?.[0].points[0]).toMatchObject({ x: 0.5, y: 0 });
  });
});

function makePathResourceData() {
  return concatBytes([
    makePathRecord(6),
    makeLengthRecord(0, 4),
    makeBezierRecord(2, 0.5, 0, 0.5, 0, 0.5, 0),
    makeBezierRecord(2, 1, 0.5, 1, 0.5, 1, 0.5),
    makeBezierRecord(2, 0.5, 1, 0.5, 1, 0.5, 1),
    makeBezierRecord(2, 0, 0.5, 0, 0.5, 0, 0.5),
    makeLengthRecord(3, 2),
    makeBezierRecord(2, 0.25, 0.5, 0.25, 0.5, 0.25, 0.5),
    makeBezierRecord(2, 0.75, 0.5, 0.75, 0.5, 0.75, 0.5),
  ]);
}

function makeResourceBlock(name: string, id: number, data: Uint8Array) {
  const nameBytes = new TextEncoder().encode(name);
  const nameFieldLength = 1 + nameBytes.length + ((1 + nameBytes.length) % 2 === 0 ? 0 : 1);
  const dataPadding = data.length % 2 === 0 ? 0 : 1;
  const block = new Uint8Array(4 + 2 + nameFieldLength + 4 + data.length + dataPadding);
  const view = new DataView(block.buffer);
  block.set([0x38, 0x42, 0x49, 0x4d], 0);
  view.setUint16(4, id, false);
  block[6] = nameBytes.length;
  block.set(nameBytes, 7);
  const sizeOffset = 6 + nameFieldLength;
  view.setUint32(sizeOffset, data.length, false);
  block.set(data, sizeOffset + 4);
  return block;
}

function makePathRecord(selector: number) {
  const record = new Uint8Array(26);
  new DataView(record.buffer).setUint16(0, selector, false);
  return record;
}

function makeLengthRecord(selector: number, points: number) {
  const record = makePathRecord(selector);
  new DataView(record.buffer).setUint16(2, points, false);
  return record;
}

function makeBezierRecord(
  selector: number,
  inX: number,
  inY: number,
  x: number,
  y: number,
  outX: number,
  outY: number,
) {
  const record = makePathRecord(selector);
  const view = new DataView(record.buffer);
  writeFixedPoint(view, 2, inY);
  writeFixedPoint(view, 6, inX);
  writeFixedPoint(view, 10, y);
  writeFixedPoint(view, 14, x);
  writeFixedPoint(view, 18, outY);
  writeFixedPoint(view, 22, outX);
  return record;
}

function writeFixedPoint(view: DataView, offset: number, value: number) {
  view.setInt32(offset, Math.round(value * 16777216), false);
}

function concatBytes(chunks: Uint8Array[]) {
  const total = chunks.reduce((sum, chunk) => sum + chunk.length, 0);
  const bytes = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.length;
  }
  return bytes;
}
