package command

const (
	commandBeginPaintStroke     int32 = 0x0400
	commandContinuePaintStroke  int32 = 0x0401
	commandEndPaintStroke       int32 = 0x0402
	commandSetForegroundColor   int32 = 0x0410
	commandSetBackgroundColor   int32 = 0x0411
	commandMagicErase           int32 = 0x0413
	commandFill                 int32 = 0x0414
	commandApplyGradient        int32 = 0x0415
	commandResetMixerBrushState int32 = 0x0416
)

type PaintBrushParams struct {
	Size             float64  `json:"size"`
	Hardness         float64  `json:"hardness"`
	Flow             float64  `json:"flow"`
	Color            [4]uint8 `json:"color"`
	BlendMode        string   `json:"blendMode,omitempty"`
	WetEdges         bool     `json:"wetEdges,omitempty"`
	Scatter          float64  `json:"scatter,omitempty"`
	Stabilizer       int      `json:"stabilizer,omitempty"`
	SampleMerged     bool     `json:"sampleMerged,omitempty"`
	AutoErase        bool     `json:"autoErase,omitempty"`
	Erase            bool     `json:"erase,omitempty"`
	EraseBackground  bool     `json:"eraseBackground,omitempty"`
	EraseTolerance   float64  `json:"eraseTolerance,omitempty"`
	MixerBrush       bool     `json:"mixerBrush,omitempty"`
	MixerMix         float64  `json:"mixerMix,omitempty"`
	MixerWetness     float64  `json:"mixerWetness,omitempty"`
	MixerLoad        float64  `json:"mixerLoad,omitempty"`
	CloneStamp       bool     `json:"cloneStamp,omitempty"`
	CloneSourceX     float64  `json:"cloneSourceX,omitempty"`
	CloneSourceY     float64  `json:"cloneSourceY,omitempty"`
	CloneAligned     bool     `json:"cloneAligned,omitempty"`
	CloneOpacity     float64  `json:"cloneOpacity,omitempty"`
	CloneLoad        float64  `json:"cloneLoad,omitempty"`
	CloneHistory     bool     `json:"cloneHistorySource,omitempty"`
	CloneHistoryIdx  int      `json:"cloneHistorySourceIndex,omitempty"`
	HistoryBrush     bool     `json:"historyBrush,omitempty"`
	HistorySourceIdx int      `json:"historySourceIndex,omitempty"`
	HistoryOpacity   float64  `json:"historyOpacity,omitempty"`
	HistoryLoad      float64  `json:"historyLoad,omitempty"`
	PressureSize     *bool    `json:"pressureSize,omitempty"`
	PressureOpacity  *bool    `json:"pressureOpacity,omitempty"`
	PressureFlow     *bool    `json:"pressureFlow,omitempty"`
}

type PaintBeginStrokePayload struct {
	X        float64          `json:"x"`
	Y        float64          `json:"y"`
	Pressure float64          `json:"pressure"`
	TiltX    float64          `json:"tiltX"`
	TiltY    float64          `json:"tiltY"`
	Brush    PaintBrushParams `json:"brush"`
}

// PaintStrokePoint is a single sample within a coalesced ContinuePaintStroke
// batch. The frontend accumulates raw pointermove samples (including
// getCoalescedEvents) and flushes them once per animation frame.
type PaintStrokePoint struct {
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Pressure float64 `json:"pressure"`
	TiltX    float64 `json:"tiltX"`
	TiltY    float64 `json:"tiltY"`
}

type PaintContinueStrokePayload struct {
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Pressure float64 `json:"pressure"`
	TiltX    float64 `json:"tiltX"`
	TiltY    float64 `json:"tiltY"`
	// Points, when non-empty, carries a coalesced batch of stroke samples to be
	// processed in order. Backward compatible: when empty the single legacy
	// X/Y/Pressure/TiltX/TiltY point above is processed as before.
	Points []PaintStrokePoint `json:"points,omitempty"`
}

type paintSetColorPayload struct {
	Color [4]uint8 `json:"color"`
}

type PaintMagicErasePayload struct {
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Tolerance    float64 `json:"tolerance"`
	Contiguous   bool    `json:"contiguous"`
	SampleMerged bool    `json:"sampleMerged"`
}

type PaintFillPayload struct {
	HasPoint     bool     `json:"hasPoint,omitempty"`
	X            float64  `json:"x,omitempty"`
	Y            float64  `json:"y,omitempty"`
	Tolerance    float64  `json:"tolerance,omitempty"`
	Contiguous   bool     `json:"contiguous,omitempty"`
	SampleMerged bool     `json:"sampleMerged,omitempty"`
	Source       string   `json:"source,omitempty"`
	Color        [4]uint8 `json:"color,omitempty"`
	CreateLayer  bool     `json:"createLayer,omitempty"`
}

type PaintGradientStopPayload struct {
	Position float64  `json:"position"`
	Color    [4]uint8 `json:"color"`
}

type PaintApplyGradientPayload struct {
	StartX      float64                    `json:"startX"`
	StartY      float64                    `json:"startY"`
	EndX        float64                    `json:"endX"`
	EndY        float64                    `json:"endY"`
	Type        string                     `json:"type"`
	Reverse     bool                       `json:"reverse,omitempty"`
	Dither      bool                       `json:"dither,omitempty"`
	CreateLayer bool                       `json:"createLayer,omitempty"`
	Stops       []PaintGradientStopPayload `json:"stops,omitempty"`
}

type PaintDeps struct {
	Decode               func(string, any) error
	BeginPaintStroke     func(PaintBeginStrokePayload) error
	ContinuePaintStroke  func(PaintContinueStrokePayload) error
	EndPaintStroke       func() error
	SetForegroundColor   func([4]uint8) error
	SetBackgroundColor   func([4]uint8) error
	MagicErase           func(PaintMagicErasePayload) error
	Fill                 func(PaintFillPayload) error
	ApplyGradient        func(PaintApplyGradientPayload) error
	ResetMixerBrushState func() error
}

func DispatchPaint(commandID int32, payloadJSON string, deps PaintDeps) (bool, error) {
	switch commandID {
	case commandBeginPaintStroke:
		var payload PaintBeginStrokePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.BeginPaintStroke(payload)

	case commandContinuePaintStroke:
		var payload PaintContinueStrokePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.ContinuePaintStroke(payload)

	case commandEndPaintStroke:
		return true, deps.EndPaintStroke()

	case commandSetForegroundColor:
		var payload paintSetColorPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetForegroundColor(payload.Color)

	case commandSetBackgroundColor:
		var payload paintSetColorPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetBackgroundColor(payload.Color)

	case commandMagicErase:
		var payload PaintMagicErasePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.MagicErase(payload)

	case commandFill:
		var payload PaintFillPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.Fill(payload)

	case commandApplyGradient:
		var payload PaintApplyGradientPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.ApplyGradient(payload)

	case commandResetMixerBrushState:
		return true, deps.ResetMixerBrushState()
	}

	return false, nil
}
