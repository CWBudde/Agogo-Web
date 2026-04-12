package engine

import (
	"encoding/base64"
	"fmt"
	"strings"
)

func (inst *instance) exportProject() (string, error) {
	if inst == nil {
		return "", fmt.Errorf("engine instance is required")
	}
	doc := inst.manager.Active()
	if doc == nil {
		return "", fmt.Errorf("no active document")
	}
	data, err := SaveProjectZip(doc, inst.history.Entries())
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func (inst *instance) importProject(payload string) (RenderResult, error) {
	if inst == nil {
		return RenderResult{}, fmt.Errorf("engine instance is required")
	}
	var doc *Document
	var warnings []string
	trimmed := strings.TrimSpace(payload)
	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil {
		switch {
		case len(decoded) >= 2 && decoded[0] == 0x50 && decoded[1] == 0x4b:
			if d, _, zipErr := LoadProjectZip(decoded); zipErr == nil {
				doc = d
			}
		case len(decoded) >= 4 && string(decoded[:4]) == "8BPS":
			if d, importWarnings, psdErr := LoadPSD(decoded); psdErr == nil {
				doc = d
				warnings = importWarnings
			}
		}
	}
	if doc == nil {
		if strings.HasPrefix(trimmed, "{") {
			var err error
			doc, _, err = LoadProject([]byte(trimmed))
			if err != nil {
				return RenderResult{}, fmt.Errorf("load project: %w", err)
			}
		} else {
			return RenderResult{}, fmt.Errorf("load project: unsupported import payload")
		}
	}
	inst.manager = newDocumentManager()
	inst.manager.Create(doc)
	inst.viewport.CenterX = float64(doc.Width) * 0.5
	inst.viewport.CenterY = float64(doc.Height) * 0.5
	inst.fitViewportToActiveDocument()
	inst.importWarnings = append([]string(nil), warnings...)
	// History is intentionally cleared on import: opening a project starts a
	// fresh undo stack regardless of any history stored in the archive.
	inst.history.Clear()
	return inst.render(), nil
}
