package command

type Domain int

const (
	DomainUnknown Domain = iota
	DomainCore
	DomainLayer
	DomainTransform
	DomainUI
	DomainSelectionPaint
	DomainFilter
	DomainPath
	DomainShape
	DomainText
)

func DomainOf(commandID int32) Domain {
	switch {
	case commandID == 0x0115 || commandID == 0x0116 || commandID == 0x011c || commandID == 0x011e || commandID == 0x0213:
		return DomainUI
	case commandID == 0x011a:
		return DomainSelectionPaint
	case commandID == 0x011b || commandID == 0x011d || (commandID >= 0x0100 && commandID <= 0x0114) || (commandID >= 0x011f && commandID <= 0x012a):
		return DomainLayer
	case commandID == 0x0001 || commandID == 0x0002 || (commandID >= 0x0010 && commandID <= 0x0017) || (commandID >= 0xffe0 && commandID <= 0xffe2) || commandID == 0xfff0 || commandID == 0xfff1 || (commandID >= 0x0117 && commandID <= 0x0119):
		return DomainCore
	case (commandID >= 0x0200 && commandID <= 0x0212) || (commandID >= 0x0400 && commandID <= 0x0418):
		return DomainSelectionPaint
	case (commandID >= 0x0300 && commandID <= 0x0309) || (commandID >= 0x0320 && commandID <= 0x0324):
		return DomainTransform
	case commandID >= 0x0500 && commandID <= 0x0505:
		return DomainFilter
	case commandID >= 0x0600 && commandID <= 0x0627:
		return DomainPath
	case commandID >= 0x0630 && commandID <= 0x0633:
		return DomainShape
	case commandID >= 0x0640 && commandID <= 0x0648:
		return DomainText
	default:
		return DomainUnknown
	}
}
