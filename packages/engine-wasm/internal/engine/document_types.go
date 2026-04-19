package engine

import docpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/document"

type (
	DocumentCore              = docpkg.Core
	DocumentCreateParams      = docpkg.CreateParams
	Background                = docpkg.Background
	DirtyRect                 = docpkg.DirtyRect
	NamedPath                 = docpkg.NamedPath
	PathMeta                  = docpkg.PathMeta
	ThumbnailEntry            = docpkg.ThumbnailEntry
	Selection                 = docpkg.Selection
	SelectionMeta             = docpkg.SelectionMeta
	SavedSelectionChannel     = docpkg.SavedSelectionChannel
	SavedSelectionChannelMeta = docpkg.SavedSelectionChannelMeta
	DocumentStylePreset       = docpkg.DocumentStylePreset
	ViewportState             = docpkg.ViewportState
	HistoryEntry              = docpkg.HistoryEntry
	RawRenderResult           = docpkg.RawRenderResult
	ProjectArchive            = docpkg.ProjectArchive
	ProjectDocumentArchive    = docpkg.ProjectDocumentArchive
	ProjectLayerArchive       = docpkg.ProjectLayerArchive
	LayerNodeMeta             = docpkg.LayerNodeMeta
)

const (
	defaultDocWidth       = docpkg.DefaultDocumentWidth
	defaultDocHeight      = docpkg.DefaultDocumentHeight
	defaultResolutionDPI  = docpkg.DefaultResolutionDPI
	defaultHistoryMax     = docpkg.DefaultHistoryMax
	defaultDevicePixelRat = docpkg.DefaultDevicePixelRatio
)

//nolint:unused // kept for package-local tests
func parseBackground(kind string) Background {
	return docpkg.ParseBackground(kind)
}

func defaultDocumentName(name string) string {
	return docpkg.DefaultDocumentName(name)
}

func newDocumentCore(params DocumentCreateParams) DocumentCore {
	return docpkg.NewCore(params)
}

func newDocumentWithCore(core DocumentCore) *Document {
	return &Document{
		Width:         core.Width,
		Height:        core.Height,
		Resolution:    core.Resolution,
		ColorMode:     core.ColorMode,
		BitDepth:      core.BitDepth,
		Background:    core.Background,
		ID:            core.ID,
		Name:          core.Name,
		CreatedAt:     core.CreatedAt,
		CreatedBy:     core.CreatedBy,
		ModifiedAt:    core.ModifiedAt,
		ActiveLayerID: core.ActiveLayerID,
		LayerRoot:     NewGroupLayer("Root"),
	}
}

func cloneNamedPaths(paths []NamedPath) []NamedPath {
	return docpkg.CloneNamedPaths(paths)
}

func cloneSelection(selection *Selection) *Selection {
	return docpkg.CloneSelection(selection)
}

func selectionEqual(a, b *Selection) bool {
	return docpkg.SelectionEqual(a, b)
}

func cloneSavedSelectionChannels(channels []SavedSelectionChannel) []SavedSelectionChannel {
	return docpkg.CloneSavedSelectionChannels(channels)
}

func savedSelectionChannelsEqual(a, b []SavedSelectionChannel) bool {
	return docpkg.SavedSelectionChannelsEqual(a, b)
}

func normalizeSelection(selection *Selection) *Selection {
	return docpkg.NormalizeSelection(selection)
}

func newSelection(width, height int) *Selection {
	return docpkg.NewSelection(width, height)
}

func newLayerMaskFromSelection(selection *Selection) *LayerMask {
	return docpkg.NewLayerMaskFromSelection(selection)
}

func cloneDocumentStylePresets(presets []DocumentStylePreset) []DocumentStylePreset {
	return docpkg.CloneDocumentStylePresets(presets)
}

func clonePresetStyles(styles []LayerStyle) []LayerStyle {
	return docpkg.ClonePresetStyles(styles)
}

func documentStylePresetsEqual(a, b []DocumentStylePreset) bool {
	return docpkg.DocumentStylePresetsEqual(a, b)
}
