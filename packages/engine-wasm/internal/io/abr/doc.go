// Package abr parses the bounds-checked subset of Adobe Photoshop brush
// libraries used by the editor.
//
// It accepts major versions 6 and 7, descriptor subversions 1 and 2, sampled
// brush records with 8-bit raw or PackBits-compressed masks, and Action
// Descriptor data used to describe computed brush presets. The package only
// decodes data; it does not register resources or render pixels.
package abr
