package engine

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"

	agglib "github.com/cwbudde/agg_go"
)

func BenchmarkImportLargeSyntheticABRLibrary(b *testing.B) {
	data := syntheticABRLibrary(64, 32, 32)
	payload, err := json.Marshal(importAbrBrushLibraryPayload{
		Data: base64.StdEncoding.EncodeToString(data), FileName: "synthetic-large.abr",
	})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for range b.N {
		inst := &instance{}
		if _, err := inst.importAbrBrushLibrary(string(payload)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSampledBrushTipStroke(b *testing.B) {
	const surfaceSize = 512
	pixels := make([]byte, surfaceSize*surfaceSize*4)
	alpha := make([]byte, 64*64)
	for y := range 64 {
		for x := range 64 {
			if dx, dy := x-32, y-32; dx*dx+dy*dy <= 32*32 {
				alpha[y*64+x] = 255
			}
		}
	}
	resource := &brushTipResource{ID: brushTipResourceID(64, 64, alpha), Width: 64, Height: 64, Alpha: alpha}
	layer := NewPixelLayer("Benchmark", LayerBounds{W: surfaceSize, H: surfaceSize}, pixels)
	renderer := agglib.NewAgg2D()
	params := BrushParams{Size: 96, Hardness: 1, Flow: 1, Color: [4]uint8{20, 40, 60, 255}, Roundness: 1}
	var scratch []byte
	b.ReportAllocs()
	b.ResetTimer()
	for index := range b.N {
		coordinate := float64(128 + index%256)
		paintDabReuseTip(renderer, layer, coordinate, coordinate, params, 0, 1, resource, &scratch)
	}
}

func syntheticABRLibrary(tipCount, width, height int) []byte {
	var library bytes.Buffer
	writeBenchmarkU16(&library, 6)
	writeBenchmarkU16(&library, 1)
	for index := range tipCount {
		pixels := bytes.Repeat([]byte{byte(index)}, width*height)
		record := syntheticABRSampleRecord(width, height, pixels)
		library.WriteString("8BIMsamp")
		writeBenchmarkU32(&library, uint32(len(record)))
		library.Write(record)
		for library.Len()%4 != 0 {
			library.WriteByte(0)
		}
	}
	return library.Bytes()
}

func syntheticABRSampleRecord(width, height int, pixels []byte) []byte {
	const uuid = "12345678-1234-1234-1234-123456789abc"
	var payload bytes.Buffer
	payload.WriteByte(byte(len(uuid)))
	payload.WriteString(uuid)
	payload.Write(make([]byte, 47-payload.Len()))
	writeBenchmarkI32(&payload, 0)
	writeBenchmarkI32(&payload, 0)
	writeBenchmarkI32(&payload, int32(height))
	writeBenchmarkI32(&payload, int32(width))
	writeBenchmarkU16(&payload, 8)
	payload.WriteByte(0)
	payload.Write(pixels)
	var record bytes.Buffer
	writeBenchmarkU32(&record, uint32(payload.Len()))
	record.Write(payload.Bytes())
	for record.Len()%4 != 0 {
		record.WriteByte(0)
	}
	return record.Bytes()
}

func writeBenchmarkU16(buffer *bytes.Buffer, value uint16) {
	_ = binary.Write(buffer, binary.BigEndian, value)
}

func writeBenchmarkU32(buffer *bytes.Buffer, value uint32) {
	_ = binary.Write(buffer, binary.BigEndian, value)
}

func writeBenchmarkI32(buffer *bytes.Buffer, value int32) {
	_ = binary.Write(buffer, binary.BigEndian, value)
}
