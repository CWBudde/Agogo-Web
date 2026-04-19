package document

const (
	DefaultDocumentWidth     = 1920
	DefaultDocumentHeight    = 1080
	DefaultResolutionDPI     = 72.0
	DefaultHistoryMax        = 50
	DefaultDevicePixelRatio  = 1.0
	DefaultDocumentColorMode = "rgb"
	DefaultDocumentBitDepth  = 8
	DefaultDocumentCreatedBy = "agogo-web"
)

type Core struct {
	Width         int        `json:"width"`
	Height        int        `json:"height"`
	Resolution    float64    `json:"resolution"`
	ColorMode     string     `json:"colorMode"`
	BitDepth      int        `json:"bitDepth"`
	Background    Background `json:"background"`
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	CreatedAt     string     `json:"createdAt"`
	CreatedBy     string     `json:"createdBy"`
	ModifiedAt    string     `json:"modifiedAt"`
	ActiveLayerID string     `json:"activeLayerId,omitempty"`
}

type CreateParams struct {
	Width         int
	Height        int
	Resolution    float64
	ColorMode     string
	BitDepth      int
	Background    string
	ID            string
	Name          string
	CreatedAt     string
	CreatedBy     string
	ModifiedAt    string
	ActiveLayerID string
}

func DefaultDocumentName(name string) string {
	if name == "" {
		return "Untitled"
	}
	return name
}

func NewCore(params CreateParams) Core {
	width := params.Width
	if width <= 0 {
		width = DefaultDocumentWidth
	}
	height := params.Height
	if height <= 0 {
		height = DefaultDocumentHeight
	}
	resolution := params.Resolution
	if resolution <= 0 {
		resolution = DefaultResolutionDPI
	}
	colorMode := params.ColorMode
	if colorMode == "" {
		colorMode = DefaultDocumentColorMode
	}
	bitDepth := params.BitDepth
	if bitDepth <= 0 {
		bitDepth = DefaultDocumentBitDepth
	}
	createdBy := params.CreatedBy
	if createdBy == "" {
		createdBy = DefaultDocumentCreatedBy
	}
	modifiedAt := params.ModifiedAt
	if modifiedAt == "" {
		modifiedAt = params.CreatedAt
	}
	return Core{
		Width:         width,
		Height:        height,
		Resolution:    resolution,
		ColorMode:     colorMode,
		BitDepth:      bitDepth,
		Background:    ParseBackground(params.Background),
		ID:            params.ID,
		Name:          DefaultDocumentName(params.Name),
		CreatedAt:     params.CreatedAt,
		CreatedBy:     createdBy,
		ModifiedAt:    modifiedAt,
		ActiveLayerID: params.ActiveLayerID,
	}
}
