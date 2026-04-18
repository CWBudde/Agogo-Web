package engine

import docpkg "github.com/cwbudde/agogo-web/packages/engine-wasm/internal/document"

type (
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
)

func parseBackground(kind string) Background {
	return docpkg.ParseBackground(kind)
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
