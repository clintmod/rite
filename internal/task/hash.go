package task

import (
	"cmp"
	"fmt"

	"github.com/clintmod/rite/internal/hash"
	"github.com/clintmod/rite/taskfile/ast"
)

func (e *Executor) GetHash(t *ast.Task) (string, error) {
	r := cmp.Or(t.Run, e.Ritefile.Run)
	var h hash.HashFunc
	switch r {
	case "always":
		h = hash.Empty
	case "once":
		h = hash.Name
	case "when_changed":
		h = hash.Hash
	default:
		return "", fmt.Errorf(`rite: invalid run "%s"`, r)
	}
	return h(t)
}
