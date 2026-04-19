package command

const (
	commandCreatePath    int32 = 0x0620
	commandDeletePath    int32 = 0x0621
	commandRenamePath    int32 = 0x0622
	commandDuplicatePath int32 = 0x0623
)

type pathCreatePayload struct {
	Name string `json:"name"`
}

type pathDeletePayload struct {
	PathIndex int `json:"pathIndex"`
}

type pathRenamePayload struct {
	PathIndex int    `json:"pathIndex"`
	Name      string `json:"name"`
}

type pathDuplicatePayload struct {
	PathIndex int `json:"pathIndex"`
}

type PathCRUDDeps struct {
	Decode        func(string, any) error
	CreatePath    func(name string) error
	DeletePath    func(pathIndex int) error
	RenamePath    func(pathIndex int, name string) error
	DuplicatePath func(pathIndex int) error
}

func DispatchPathCRUD(commandID int32, payloadJSON string, deps PathCRUDDeps) (bool, error) {
	switch commandID {
	case commandCreatePath:
		var payload pathCreatePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.CreatePath(payload.Name)

	case commandDeletePath:
		var payload pathDeletePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DeletePath(payload.PathIndex)

	case commandRenamePath:
		var payload pathRenamePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.RenamePath(payload.PathIndex, payload.Name)

	case commandDuplicatePath:
		var payload pathDuplicatePayload
		if err := deps.Decode(payloadJSON, &payload); err != nil {
			return true, err
		}
		return true, deps.DuplicatePath(payload.PathIndex)
	}

	return false, nil
}
