package engine

import "fmt"

func (inst *instance) exportProject() (string, error) {
	if inst == nil {
		return "", fmt.Errorf("engine instance is required")
	}
	doc := inst.manager.Active()
	if doc == nil {
		return "", fmt.Errorf("no active document")
	}
	data, err := SaveProject(doc, inst.history.Entries())
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (inst *instance) importProject(payload string) (RenderResult, error) {
	if inst == nil {
		return RenderResult{}, fmt.Errorf("engine instance is required")
	}
	doc, _, err := LoadProject([]byte(payload))
	if err != nil {
		return RenderResult{}, err
	}
	inst.manager = newDocumentManager()
	inst.manager.Create(doc)
	inst.viewport.CenterX = float64(doc.Width) / 2
	inst.viewport.CenterY = float64(doc.Height) / 2
	inst.fitViewportToActiveDocument()
	inst.history.Clear()
	return inst.render(), nil
}
