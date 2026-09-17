package abr

import (
	"errors"
	"fmt"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/io/descriptor"
)

// descriptorLimits projects the ABR limit set onto the shared parser's.
func (l Limits) descriptorLimits() descriptor.Limits {
	return descriptor.Limits{
		MaxDepth:       l.MaxDescriptorDepth,
		MaxItems:       l.MaxDescriptorItems,
		MaxStringBytes: l.MaxStringBytes,
		MaxDataBytes:   l.MaxDataBytes,
	}
}

// translateDescriptorError restates a shared-parser error in ABR's own error
// vocabulary, so callers keep matching on abr.ErrMalformed and friends rather
// than having to know which package parsed the bytes.
func translateDescriptorError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, descriptor.ErrLimit):
		return fmt.Errorf("%w: %v", ErrLimit, err)
	case errors.Is(err, descriptor.ErrUnsupported):
		return fmt.Errorf("%w: %v", ErrUnsupported, err)
	default:
		return fmt.Errorf("%w: %v", ErrMalformed, err)
	}
}

func parseDescriptorSection(payload []byte, limits Limits) (Descriptor, error) {
	d, err := descriptor.ParseVersioned(payload, limits.descriptorLimits())
	if err != nil {
		return Descriptor{}, translateDescriptorError(err)
	}
	return d, nil
}
