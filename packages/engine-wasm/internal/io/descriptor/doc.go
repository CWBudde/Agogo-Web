// Package descriptor reads Adobe Action Descriptors, the key/value structure
// Photoshop embeds in ABR brush libraries and in PSD tagged blocks such as
// lfx2 (layer effects) and TySh (text layers).
//
// The wire format is the same in both files, so this package is shared rather
// than duplicated: internal/io/abr and internal/io/psd both parse it, and a
// second implementation of the same format would be two things to keep in step.
//
// # Unimplemented value types
//
// ObAr, obj, UnFl, comp, prop, Enmr, rele, Idnt, indx and name are not
// implemented. They return ErrUnsupported naming the type rather than being
// skipped, because a descriptor value has no length prefix: a reader that does
// not understand a type cannot step over it, and guessing desynchronises
// everything after it. None of them appears in lfx2 or TySh. When a fixture
// turns up carrying one, it lands with that fixture.
//
// # Trailing NUL
//
// Photoshop NUL-terminates TEXT values and counts the terminator in the
// character count. Value.String keeps the bytes verbatim, so a parsed value
// still carries what the file held and can be re-emitted unchanged; callers
// that want the string a user would recognise call Value.Text.
package descriptor
