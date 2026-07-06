package text

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"codeberg.org/go-fonts/dejavu/dejavusans"
	"codeberg.org/go-fonts/dejavu/dejavusansbold"
	"codeberg.org/go-fonts/dejavu/dejavusansboldoblique"
	"codeberg.org/go-fonts/dejavu/dejavusansoblique"
	"golang.org/x/image/font/sfnt"
)

// FallbackFamily is the family every unresolvable request falls back to.
// The default registry always carries all four styles of it.
const FallbackFamily = "DejaVu Sans"

// FaceKey identifies a registered face. Family is normalized (lowercase,
// trimmed) so lookups are case- and whitespace-insensitive.
type FaceKey struct {
	Family string
	Bold   bool
	Italic bool
}

// faceKeyFor builds the normalized lookup key for a family/style request.
func faceKeyFor(family string, bold, italic bool) FaceKey {
	return FaceKey{Family: normalizeFamily(family), Bold: bold, Italic: italic}
}

// normalizeFamily lowercases and trims a family name for use as a map key.
func normalizeFamily(family string) string {
	return strings.ToLower(strings.TrimSpace(family))
}

// FamilyInfo describes one registered family for UI enumeration. Styles
// holds human-readable style names ("Regular", "Bold", "Italic",
// "Bold Italic") in that canonical order.
type FamilyInfo struct {
	Family string
	Styles []string
}

type faceEntry struct {
	face    *Face
	display string // family name as passed to Register, trimmed
}

// Registry maps (family, bold, italic) keys to parsed faces. All methods
// are safe for concurrent use.
type Registry struct {
	mu    sync.RWMutex
	faces map[FaceKey]*faceEntry
}

// NewRegistry returns an empty registry. Note that Resolve on a registry
// without a registered FallbackFamily regular face can return nil; use
// DefaultRegistry for the always-non-nil guarantee.
func NewRegistry() *Registry {
	return &Registry{faces: make(map[FaceKey]*faceEntry)}
}

// Register parses data as an SFNT font (TTF/OTF) and stores it under the
// given family and style. Invalid font data returns an error and leaves the
// registry unchanged. Re-registering an existing key replaces the face.
func (r *Registry) Register(family string, bold, italic bool, data []byte) error {
	fnt, err := sfnt.Parse(data)
	if err != nil {
		return fmt.Errorf("text: parse font %q: %w", family, err)
	}
	key := faceKeyFor(family, bold, italic)
	if key.Family == "" {
		return fmt.Errorf("text: empty font family name")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.faces[key] = &faceEntry{face: newFace(fnt), display: strings.TrimSpace(family)}
	return nil
}

// Resolve returns the face for a family/style request using the fallback
// chain: exact (family, bold, italic) -> (family, regular) ->
// (FallbackFamily, same style) -> (FallbackFamily, regular). An empty
// family, "system-ui", or an unknown family resolves within FallbackFamily
// while preserving the requested style. Resolve never returns nil on the
// default registry (DejaVu Sans regular is always registered); on a custom
// registry missing every fallback it returns nil.
func (r *Registry) Resolve(family string, bold, italic bool) *Face {
	norm := normalizeFamily(family)
	fallback := normalizeFamily(FallbackFamily)
	if norm == "" || norm == "system-ui" {
		norm = fallback
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, key := range []FaceKey{
		{Family: norm, Bold: bold, Italic: italic},
		{Family: norm},
		{Family: fallback, Bold: bold, Italic: italic},
		{Family: fallback},
	} {
		if e, ok := r.faces[key]; ok {
			return e.face
		}
	}
	return nil
}

// styleName maps a bold/italic pair to its display style name.
func styleName(bold, italic bool) string {
	switch {
	case bold && italic:
		return "Bold Italic"
	case bold:
		return "Bold"
	case italic:
		return "Italic"
	default:
		return "Regular"
	}
}

// styleRank orders styles canonically: Regular, Bold, Italic, Bold Italic.
func styleRank(name string) int {
	switch name {
	case "Regular":
		return 0
	case "Bold":
		return 1
	case "Italic":
		return 2
	default:
		return 3
	}
}

// List enumerates registered families sorted by family name, each with its
// available styles in canonical order (Regular, Bold, Italic, Bold Italic).
func (r *Registry) List() []FamilyInfo {
	r.mu.RLock()
	byFamily := make(map[string]*FamilyInfo)
	for key, entry := range r.faces {
		info, ok := byFamily[key.Family]
		if !ok {
			info = &FamilyInfo{Family: entry.display}
			byFamily[key.Family] = info
		}
		info.Styles = append(info.Styles, styleName(key.Bold, key.Italic))
	}
	r.mu.RUnlock()

	infos := make([]FamilyInfo, 0, len(byFamily))
	for _, info := range byFamily {
		sort.Slice(info.Styles, func(i, j int) bool {
			return styleRank(info.Styles[i]) < styleRank(info.Styles[j])
		})
		infos = append(infos, *info)
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Family < infos[j].Family })
	return infos
}

var (
	defaultRegistryOnce sync.Once
	defaultRegistry     *Registry
)

// DefaultRegistry returns the process-wide registry, pre-populated with the
// four embedded DejaVu Sans styles. Resolve on it never returns nil.
func DefaultRegistry() *Registry {
	defaultRegistryOnce.Do(func() {
		r := NewRegistry()
		embedded := []struct {
			bold, italic bool
			data         []byte
		}{
			{false, false, dejavusans.TTF},
			{true, false, dejavusansbold.TTF},
			{false, true, dejavusansoblique.TTF},
			{true, true, dejavusansboldoblique.TTF},
		}
		for _, e := range embedded {
			if err := r.Register(FallbackFamily, e.bold, e.italic, e.data); err != nil {
				// The embedded fonts are known-good; failing to parse them
				// is a programming or vendoring error, not a runtime state.
				panic(fmt.Sprintf("text: embedded %s %s failed to parse: %v",
					FallbackFamily, styleName(e.bold, e.italic), err))
			}
		}
		defaultRegistry = r
	})
	return defaultRegistry
}
