package engine

// --- Payloads ---

// CreatePathPayload is the JSON payload for commandCreatePath.
type CreatePathPayload struct {
	Name string `json:"name"`
}

// DeletePathPayload is the JSON payload for commandDeletePath.
type DeletePathPayload struct {
	PathIndex int `json:"pathIndex"`
}

// RenamePathPayload is the JSON payload for commandRenamePath.
type RenamePathPayload struct {
	PathIndex int    `json:"pathIndex"`
	Name      string `json:"name"`
}

// DuplicatePathPayload is the JSON payload for commandDuplicatePath.
type DuplicatePathPayload struct {
	PathIndex int `json:"pathIndex"`
}

// MakeSelectionFromPathPayload is the JSON payload for commandMakeSelectionFromPath
// and commandRasterizePath.
type MakeSelectionFromPathPayload struct {
	PathIndex     *int    `json:"pathIndex,omitempty"` // nil = active path
	FeatherRadius float64 `json:"featherRadius,omitempty"`
	AntiAlias     bool    `json:"antiAlias,omitempty"`
}

// FillPathPayload is the JSON payload for commandFillPath.
type FillPathPayload struct {
	PathIndex *int     `json:"pathIndex,omitempty"` // nil = active path
	Color     [4]uint8 `json:"color,omitempty"`     // if zero, use foreground color
}

// StrokePathPayload is the JSON payload for commandStrokePath.
type StrokePathPayload struct {
	PathIndex *int     `json:"pathIndex,omitempty"` // nil = active path
	ToolWidth float64  `json:"toolWidth,omitempty"`
	Color     [4]uint8 `json:"color,omitempty"` // if zero, use foreground color
}

// --- Document methods ---
