package engine

import (
	"fmt"
	"sync/atomic"
)

// documentSession owns state that must follow a document when another tab is
// activated. Canvas dimensions and device-pixel ratio remain workspace-wide
// and are deliberately copied from the live viewport when a session is loaded.
type documentSession struct {
	history             *HistoryStack
	viewport            ViewportState
	lastTransform       *LastTransformRecord
	lastFilter          *lastFilterState
	preFadeSnapshot     *fadeSnapshot
	importWarnings      []string
	savedContentVersion int64
	hasSavedBaseline    bool
}

func (inst *instance) ensureDocumentSessions() {
	if inst.documentSessions == nil {
		inst.documentSessions = make(map[string]*documentSession)
	}
}

func (inst *instance) saveActiveDocumentSession() {
	id := inst.manager.ActiveID()
	if id == "" {
		return
	}
	inst.ensureDocumentSessions()
	session := inst.documentSessions[id]
	if session == nil {
		session = &documentSession{}
		inst.documentSessions[id] = session
	}
	session.history = inst.history
	session.viewport = inst.viewport
	session.lastTransform = inst.lastTransform
	session.lastFilter = inst.lastFilter
	session.preFadeSnapshot = inst.preFadeSnapshot
	session.importWarnings = append(session.importWarnings[:0], inst.importWarnings...)
}

func (inst *instance) loadDocumentSession(id string) {
	inst.ensureDocumentSessions()
	session := inst.documentSessions[id]
	if session == nil {
		session = &documentSession{
			history:  newHistoryStack(defaultHistoryMax),
			viewport: inst.viewport,
		}
		inst.documentSessions[id] = session
	}
	if session.history == nil {
		session.history = newHistoryStack(defaultHistoryMax)
	}
	canvasW := inst.viewport.CanvasW
	canvasH := inst.viewport.CanvasH
	dpr := inst.viewport.DevicePixelRatio
	inst.history = session.history
	inst.viewport = session.viewport
	inst.viewport.CanvasW = canvasW
	inst.viewport.CanvasH = canvasH
	inst.viewport.DevicePixelRatio = dpr
	inst.lastTransform = session.lastTransform
	inst.lastFilter = session.lastFilter
	inst.preFadeSnapshot = session.preFadeSnapshot
	inst.importWarnings = append(inst.importWarnings[:0], session.importWarnings...)
}

func (inst *instance) createDocumentSession(doc *Document, modified bool) {
	inst.saveActiveDocumentSession()
	inst.manager.Create(doc)
	inst.history = newHistoryStack(defaultHistoryMax)
	inst.lastTransform = nil
	inst.lastFilter = nil
	inst.preFadeSnapshot = nil
	inst.importWarnings = nil
	inst.viewport.CenterX = float64(doc.Width) * 0.5
	inst.viewport.CenterY = float64(doc.Height) * 0.5
	inst.viewport.Rotation = 0
	inst.fitViewportToActiveDocument()
	inst.ensureDocumentSessions()
	inst.documentSessions[doc.ID] = &documentSession{
		history:             inst.history,
		viewport:            inst.viewport,
		savedContentVersion: doc.ContentVersion,
		hasSavedBaseline:    !modified,
	}
}

func (inst *instance) prepareDocumentSwitch() error {
	if inst.paintStroke != nil {
		if err := inst.handleEndPaintStroke(); err != nil {
			return err
		}
	}
	if inst.textEdit.layerID != "" {
		if err := inst.commitTextEdit(); err != nil {
			return err
		}
	}
	if inst.filterPreview != nil {
		if _, err := inst.handleCancelFilterPreview(); err != nil {
			return err
		}
	}
	inst.freeTransform = nil
	inst.crop = nil
	inst.pointer = pointerDragState{}
	inst.maskEditLayerID = ""
	inst.editingVectorLayerID = ""
	inst.pathTool = newPathToolState()
	inst.resetMixerBrushState()
	inst.resetCloneStampState()
	return nil
}

func (inst *instance) switchDocument(id string) error {
	if id == "" {
		return fmt.Errorf("document id is required")
	}
	if id == inst.manager.ActiveID() {
		return nil
	}
	if err := inst.prepareDocumentSwitch(); err != nil {
		return err
	}
	inst.saveActiveDocumentSession()
	if err := inst.manager.Switch(id); err != nil {
		return err
	}
	inst.loadDocumentSession(id)
	inst.invalidateRenderCaches()
	return nil
}

func (inst *instance) closeDocument(id string) error {
	if id == "" {
		id = inst.manager.ActiveID()
	}
	if id == "" {
		return nil
	}
	wasActive := id == inst.manager.ActiveID()
	if wasActive {
		if err := inst.prepareDocumentSwitch(); err != nil {
			return err
		}
		inst.saveActiveDocumentSession()
	}
	if err := inst.manager.Close(id); err != nil {
		return err
	}
	delete(inst.documentSessions, id)
	if next := inst.manager.ActiveID(); next != "" {
		inst.loadDocumentSession(next)
	} else {
		inst.history = newHistoryStack(defaultHistoryMax)
		inst.lastTransform = nil
		inst.lastFilter = nil
		inst.preFadeSnapshot = nil
		inst.importWarnings = nil
	}
	inst.invalidateRenderCaches()
	return nil
}

func (inst *instance) markDocumentSaved(id string) error {
	if id == "" {
		id = inst.manager.ActiveID()
	}
	doc := inst.manager.Get(id)
	if doc == nil {
		return fmt.Errorf("document %q not found", id)
	}
	inst.ensureDocumentSessions()
	session := inst.documentSessions[id]
	if session == nil {
		session = &documentSession{history: newHistoryStack(defaultHistoryMax)}
		inst.documentSessions[id] = session
	}
	session.savedContentVersion = doc.ContentVersion
	session.hasSavedBaseline = true
	return nil
}

func (inst *instance) documentSummaries() []DocumentSummary {
	ids := inst.manager.IDs()
	result := make([]DocumentSummary, 0, len(ids))
	for _, id := range ids {
		doc := inst.manager.Get(id)
		if doc == nil {
			continue
		}
		session := inst.documentSessions[id]
		modified := true
		if session != nil && session.hasSavedBaseline {
			modified = session.savedContentVersion != doc.ContentVersion
		}
		result = append(result, DocumentSummary{
			ID:       id,
			Name:     doc.Name,
			Width:    doc.Width,
			Height:   doc.Height,
			Active:   id == inst.manager.ActiveID(),
			Modified: modified,
		})
	}
	return result
}

func (inst *instance) canTransformAgainNow() bool {
	if inst.lastTransform == nil || inst.paintStroke != nil || inst.filterPreview != nil {
		return false
	}
	if inst.freeTransform != nil || inst.crop != nil || inst.textEdit.layerID != "" || inst.editingVectorLayerID != "" {
		return false
	}
	doc := inst.manager.activeMut()
	if doc == nil {
		return false
	}
	_, ok := doc.ActiveLayer().(*PixelLayer)
	return ok
}

func (inst *instance) invalidateRenderCaches() {
	inst.hasCachedRawFrame = false
	inst.hasCachedAnimBase = false
	inst.cachedDocID = ""
	inst.cachedDocContentVersion = 0
}

func (inst *instance) uniqueImportedDocumentID() string {
	for {
		id := fmt.Sprintf("doc-%04d", atomic.AddInt64(&nextDocID, 1))
		if inst.manager.Get(id) == nil {
			return id
		}
	}
}
