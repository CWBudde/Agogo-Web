package engine

import (
	"encoding/base64"
	"fmt"
	"math"

	agglib "github.com/cwbudde/agg_go"
)

const navigatorMaxDimension = 1024

type NavigatorThumbnailPayload struct {
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Background string `json:"background,omitempty"`
}

type navigatorThumbnailKey struct {
	documentID     string
	contentVersion int64
	width          int
	height         int
	background     string
}

func (inst *instance) getNavigatorThumbnail(payload NavigatorThumbnailPayload) (*NavigatorThumbnail, error) {
	doc := inst.manager.activeMut()
	if doc == nil {
		return nil, fmt.Errorf("no active document")
	}
	requestedW := maxInt(1, minInt(payload.Width, navigatorMaxDimension))
	requestedH := maxInt(1, minInt(payload.Height, navigatorMaxDimension))
	background := stringValueOrDefault(payload.Background, "transparent")
	key := navigatorThumbnailKey{
		documentID: doc.ID, contentVersion: doc.ContentVersion,
		width: requestedW, height: requestedH, background: background,
	}
	if inst.navigatorCache != nil && inst.navigatorCacheKey == key {
		copy := *inst.navigatorCache
		return &copy, nil
	}

	scale := math.Min(float64(requestedW)/float64(maxInt(doc.Width, 1)), float64(requestedH)/float64(maxInt(doc.Height, 1)))
	width := maxInt(1, int(math.Round(float64(doc.Width)*scale)))
	height := maxInt(1, int(math.Round(float64(doc.Height)*scale)))
	surface, err := inst.compositeSurfaceChecked(doc)
	if err != nil {
		return nil, err
	}
	dest := make([]byte, width*height*4)
	renderer := agglib.NewAgg2D()
	renderer.Attach(dest, width, height, width*4)
	renderer.ImageFilter(agglib.Bilinear)
	image := agglib.NewImage(surface, doc.Width, doc.Height, doc.Width*4)
	if err := renderer.TransformImageSimple(image, 0, 0, float64(width), float64(height)); err != nil {
		return nil, fmt.Errorf("scale navigator thumbnail: %w", err)
	}
	thumbnail := &NavigatorThumbnail{
		DocumentID:      doc.ID,
		ContentVersion:  doc.ContentVersion,
		RequestedWidth:  requestedW,
		RequestedHeight: requestedH,
		Width:           width,
		Height:          height,
		Background:      background,
		RGBA:            base64.StdEncoding.EncodeToString(dest),
	}
	inst.navigatorCacheKey = key
	inst.navigatorCache = thumbnail
	copy := *thumbnail
	return &copy, nil
}
