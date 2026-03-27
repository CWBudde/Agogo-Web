package engine

import (
	"encoding/base64"
	"fmt"
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
	// Try base64-encoded ZIP first
	if zipBytes, err := base64.StdEncoding.DecodeString(payload); err == nil {
		if d, _, zipErr := LoadProjectZip(zipBytes); zipErr == nil {
			doc = d
		}
	}
	// Fallback: try legacy JSON
	if doc == nil {
		var err error
		doc, _, err = LoadProject([]byte(payload))
		if err != nil {
			return RenderResult{}, fmt.Errorf("load project: %w", err)
		}
	}
	inst.manager = newDocumentManager()
	inst.manager.Create(doc)
	inst.viewport.CenterX = float64(doc.Width) / 2
	inst.viewport.CenterY = float64(doc.Height) / 2
	inst.fitViewportToActiveDocument()
	// History is intentionally cleared on import: opening a project starts a
	// fresh undo stack regardless of any history stored in the archive.
	inst.history.Clear()
	return inst.render(), nil
}
