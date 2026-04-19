package document

import (
	"fmt"

	"github.com/cwbudde/agogo-web/packages/engine-wasm/internal/model"
)

func CreatePath(paths []NamedPath, name string) ([]NamedPath, int) {
	if name == "" {
		name = fmt.Sprintf("Path %d", len(paths)+1)
	}
	paths = append(paths, NamedPath{Name: name})
	return paths, len(paths) - 1
}

func DeletePath(paths []NamedPath, activePathIdx, index int) ([]NamedPath, int, error) {
	if index < 0 || index >= len(paths) {
		return paths, activePathIdx, fmt.Errorf("path index %d out of range", index)
	}
	paths = append(paths[:index], paths[index+1:]...)
	if activePathIdx >= len(paths) {
		activePathIdx = len(paths) - 1
	}
	return paths, activePathIdx, nil
}

func RenamePath(paths []NamedPath, index int, name string) error {
	if index < 0 || index >= len(paths) {
		return fmt.Errorf("path index %d out of range", index)
	}
	paths[index].Name = name
	return nil
}

func DuplicatePath(paths []NamedPath, index int) ([]NamedPath, int, error) {
	if index < 0 || index >= len(paths) {
		return paths, -1, fmt.Errorf("path index %d out of range", index)
	}
	src := paths[index]
	dup := NamedPath{
		Name: src.Name + " copy",
		Path: *model.ClonePath(&src.Path),
	}
	paths = append(paths, NamedPath{})
	copy(paths[index+2:], paths[index+1:])
	paths[index+1] = dup
	return paths, index + 1, nil
}
