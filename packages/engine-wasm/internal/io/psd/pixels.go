package psd

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
)

func (p *Parser) ParseCompositeImageData(header Header) ([]byte, error) {
	if err := validateSupportedDepth(header.Depth); err != nil {
		return nil, err
	}
	pixelsPerPlane, err := checkedPixelCount(header.Width, header.Height)
	if err != nil {
		return nil, err
	}
	if header.Channels <= 0 || header.Channels > PSDMaxChannels || (header.Channels > 0 && pixelsPerPlane > maxPSDDecodedByteSize/header.Channels) {
		return nil, fmt.Errorf("invalid composite channel count or decoded size")
	}
	compression, err := p.readUint16()
	if err != nil {
		return nil, err
	}
	if pixelsPerPlane == 0 {
		return nil, nil
	}
	planes := make([][]byte, header.Channels)
	switch compression {
	case CompressionRaw:
		for i := 0; i < header.Channels; i++ {
			planes[i], err = p.readBytes(pixelsPerPlane)
			if err != nil {
				return nil, err
			}
		}
	case CompressionRLE:
		counts := make([]int, header.Channels*header.Height)
		for i := range counts {
			if header.PSB {
				value, err := p.readUint32()
				if err != nil {
					return nil, err
				}
				counts[i] = int(value)
			} else {
				value, err := p.readUint16()
				if err != nil {
					return nil, err
				}
				counts[i] = int(value)
			}
		}
		for planeIndex := 0; planeIndex < header.Channels; planeIndex++ {
			plane := make([]byte, 0, pixelsPerPlane)
			for row := 0; row < header.Height; row++ {
				size := counts[planeIndex*header.Height+row]
				encoded, err := p.readBytes(size)
				if err != nil {
					return nil, err
				}
				decoded, err := DecodePackBits(encoded, header.Width)
				if err != nil {
					return nil, err
				}
				plane = append(plane, decoded...)
			}
			planes[planeIndex] = plane
		}
	case CompressionZip, CompressionZipPrediction:
		compressed, err := p.readBytes(p.r.Len())
		if err != nil {
			return nil, err
		}
		flat, err := decodeZipImageData(compressed, header.Channels, pixelsPerPlane, header.Width, header.Height, compression == CompressionZipPrediction)
		if err != nil {
			return nil, err
		}
		for i := 0; i < header.Channels; i++ {
			plane := flat[i*pixelsPerPlane : (i+1)*pixelsPerPlane]
			planes[i] = append([]byte(nil), plane...)
		}
	default:
		return nil, fmt.Errorf("unsupported PSD composite compression %d", compression)
	}
	return compositePlanesToRGBA(header.ColorMode, planes, pixelsPerPlane)
}

func parseChannelImageData(reader *bytes.Reader, psb bool, declaredLength uint64, width, height int) ([]byte, error) {
	data, err := readBytesFrom(reader, int(declaredLength))
	if err != nil {
		return nil, err
	}
	channelReader := bytes.NewReader(data)
	compression, err := readUint16From(channelReader)
	if err != nil {
		return nil, err
	}
	switch compression {
	case CompressionRaw:
		return readBytesFrom(channelReader, width*height)
	case CompressionRLE:
		if width <= 0 || height <= 0 {
			return nil, nil
		}
		counts := make([]int, height)
		for i := range counts {
			if psb {
				value, err := readUint32From(channelReader)
				if err != nil {
					return nil, err
				}
				counts[i] = int(value)
			} else {
				value, err := readUint16From(channelReader)
				if err != nil {
					return nil, err
				}
				counts[i] = int(value)
			}
		}
		decoded := make([]byte, 0, width*height)
		for _, count := range counts {
			rowData, err := readBytesFrom(channelReader, count)
			if err != nil {
				return nil, err
			}
			row, err := DecodePackBits(rowData, width)
			if err != nil {
				return nil, err
			}
			decoded = append(decoded, row...)
		}
		if len(decoded) != width*height {
			return nil, fmt.Errorf("decoded RLE channel length %d, want %d", len(decoded), width*height)
		}
		return decoded, nil
	case CompressionZip, CompressionZipPrediction:
		compressed, err := readBytesFrom(channelReader, channelReader.Len())
		if err != nil {
			return nil, err
		}
		return decodeZipChannel(compressed, width, height, compression == CompressionZipPrediction)
	default:
		return nil, fmt.Errorf("unsupported PSD layer compression %d", compression)
	}
}

func decodeZipChannel(data []byte, width, height int, withPrediction bool) ([]byte, error) {
	pixelCount, err := checkedPixelCount(width, height)
	if err != nil {
		return nil, err
	}
	pixels, err := decodeZipPayload(data, pixelCount)
	if err != nil {
		return nil, err
	}
	if len(pixels) != pixelCount {
		return nil, fmt.Errorf("decoded zip channel length %d, want %d", len(pixels), pixelCount)
	}
	if withPrediction {
		applyZipPredictionInPlace(pixels, width, height)
	}
	return pixels, nil
}

func decodeZipImageData(compressed []byte, channelCount, pixelCount, width, height int, withPrediction bool) ([]byte, error) {
	if channelCount < 0 || pixelCount < 0 || (channelCount > 0 && pixelCount > maxPSDDecodedByteSize/channelCount) {
		return nil, fmt.Errorf("invalid zip image dimensions")
	}
	expected := channelCount * pixelCount
	decoded, err := decodeZipPayload(compressed, expected)
	if err != nil {
		return nil, err
	}
	if len(decoded) != expected {
		return nil, fmt.Errorf("decoded zip image length %d, want %d", len(decoded), expected)
	}
	if withPrediction {
		for i := 0; i < channelCount; i++ {
			start := i * pixelCount
			end := start + pixelCount
			applyZipPredictionInPlace(decoded[start:end], width, height)
		}
	}
	return decoded, nil
}

func decodeZipPayload(data []byte, expectedLen int) ([]byte, error) {
	if expectedLen < 0 || expectedLen > maxPSDDecodedByteSize {
		return nil, fmt.Errorf("invalid zip decoded length %d", expectedLen)
	}
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to init zip stream: %w", err)
	}
	decoded, err := io.ReadAll(io.LimitReader(zr, int64(expectedLen)+1))
	if err != nil {
		if closeErr := zr.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to decode zip stream: %w (close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to decode zip stream: %w", err)
	}
	if err := zr.Close(); err != nil {
		return nil, fmt.Errorf("failed to close zip stream: %w", err)
	}
	if len(decoded) > expectedLen {
		return nil, fmt.Errorf("decoded zip payload exceeds expected length %d", expectedLen)
	}
	return decoded, nil
}

func checkedPixelCount(width, height int) (int, error) {
	if width < 0 || height < 0 || (width > 0 && height > maxPSDDecodedByteSize/width) {
		return 0, fmt.Errorf("invalid pixel dimensions %dx%d", width, height)
	}
	return width * height, nil
}

func applyZipPredictionInPlace(data []byte, width, height int) {
	if width <= 0 || height <= 0 {
		return
	}
	for row := 0; row < height; row++ {
		rowStart := row * width
		rowEnd := rowStart + width
		if rowEnd > len(data) {
			break
		}
		for col := 1; col < width && rowStart+col < rowEnd; col++ {
			data[rowStart+col] = data[rowStart+col] + data[rowStart+col-1]
		}
	}
}

func compositePlanesToRGBA(colorMode int, planes [][]byte, pixelCount int) ([]byte, error) {
	if pixelCount == 0 {
		return nil, nil
	}
	rgba := make([]byte, pixelCount*4)
	switch colorMode {
	case ColorModeRGB:
		if len(planes) < 3 {
			return nil, fmt.Errorf("composite image missing RGB planes")
		}
		for i := 0; i < pixelCount; i++ {
			rgba[i*4] = planes[0][i]
			rgba[i*4+1] = planes[1][i]
			rgba[i*4+2] = planes[2][i]
			rgba[i*4+3] = 255
			if len(planes) > 3 && len(planes[3]) == pixelCount {
				rgba[i*4+3] = planes[3][i]
			}
		}
	case ColorModeGrayscale:
		if len(planes) == 0 {
			return nil, fmt.Errorf("composite image missing grayscale plane")
		}
		for i := 0; i < pixelCount; i++ {
			value := planes[0][i]
			rgba[i*4] = value
			rgba[i*4+1] = value
			rgba[i*4+2] = value
			rgba[i*4+3] = 255
			if len(planes) > 1 && len(planes[1]) == pixelCount {
				rgba[i*4+3] = planes[1][i]
			}
		}
	default:
		return nil, fmt.Errorf("unsupported composite color mode %d", colorMode)
	}
	return rgba, nil
}
