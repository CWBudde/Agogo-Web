package command

const (
	commandAddTextLayer      int32 = 0x0640
	commandSetTextContent    int32 = 0x0641
	commandSetTextStyle      int32 = 0x0642
	commandEnterTextEditMode int32 = 0x0643
	commandTextEditInput     int32 = 0x0644
	commandCommitTextEdit    int32 = 0x0645
	commandConvertTextToPath int32 = 0x0646
)

type TextAddLayerPayload struct {
	X        float64  `json:"x"`
	Y        float64  `json:"y"`
	FontSize float64  `json:"fontSize,omitempty"`
	Color    [4]uint8 `json:"color,omitempty"`
	TextType string   `json:"textType,omitempty"`
}

type TextSetContentPayload struct {
	LayerID string `json:"layerId"`
	Text    string `json:"text"`
}

type TextSetStylePayload struct {
	LayerID       string    `json:"layerId"`
	FontFamily    *string   `json:"fontFamily,omitempty"`
	FontStyle     *string   `json:"fontStyle,omitempty"`
	FontSize      *float64  `json:"fontSize,omitempty"`
	Bold          *bool     `json:"bold,omitempty"`
	Italic        *bool     `json:"italic,omitempty"`
	Color         *[4]uint8 `json:"color,omitempty"`
	Alignment     *string   `json:"alignment,omitempty"`
	Leading       *float64  `json:"leading,omitempty"`
	TextType      *string   `json:"textType,omitempty"`
	Tracking      *float64  `json:"tracking,omitempty"`
	AntiAlias     *string   `json:"antiAlias,omitempty"`
	Kerning       *float64  `json:"kerning,omitempty"`
	Language      *string   `json:"language,omitempty"`
	BaselineShift *float64  `json:"baselineShift,omitempty"`
	Superscript   *bool     `json:"superscript,omitempty"`
	Subscript     *bool     `json:"subscript,omitempty"`
	Orientation   *string   `json:"orientation,omitempty"`
	Underline     *bool     `json:"underline,omitempty"`
	Strikethrough *bool     `json:"strikethrough,omitempty"`
	AllCaps       *bool     `json:"allCaps,omitempty"`
	SmallCaps     *bool     `json:"smallCaps,omitempty"`
	IndentLeft    *float64  `json:"indentLeft,omitempty"`
	IndentRight   *float64  `json:"indentRight,omitempty"`
	IndentFirst   *float64  `json:"indentFirst,omitempty"`
	SpaceBefore   *float64  `json:"spaceBefore,omitempty"`
	SpaceAfter    *float64  `json:"spaceAfter,omitempty"`
}

type textLayerIDPayload struct {
	LayerID string `json:"layerId"`
}

type textEditInputPayload struct {
	Text string `json:"text"`
}

type TextDeps struct {
	Decode            func(string, any) error
	AddTextLayer      func(TextAddLayerPayload) error
	SetTextContent    func(TextSetContentPayload) error
	SetTextStyle      func(TextSetStylePayload) error
	EnterTextEditMode func(layerID string) error
	TextEditInput     func(text string) error
	CommitTextEdit    func() error
	ConvertTextToPath func(layerID string) error
}

func DispatchText(commandID int32, payloadJSON string, deps TextDeps) (bool, error) {
	switch commandID {
	case commandAddTextLayer:
		var payload TextAddLayerPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.AddTextLayer(payload)
	case commandSetTextContent:
		var payload TextSetContentPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetTextContent(payload)
	case commandSetTextStyle:
		var payload TextSetStylePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.SetTextStyle(payload)
	case commandEnterTextEditMode:
		var payload textLayerIDPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.EnterTextEditMode(payload.LayerID)
	case commandTextEditInput:
		var payload textEditInputPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.TextEditInput(payload.Text)
	case commandCommitTextEdit:
		return true, deps.CommitTextEdit()
	case commandConvertTextToPath:
		var payload textLayerIDPayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.ConvertTextToPath(payload.LayerID)
	default:
		return false, nil
	}
}
