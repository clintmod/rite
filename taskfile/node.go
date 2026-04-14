package taskfile

import (
	"strings"

	"github.com/clintmod/rite/errors"
	"github.com/clintmod/rite/internal/fsext"
)

type Node interface {
	Read() ([]byte, error)
	Parent() Node
	Location() string
	Dir() string
	Checksum() string
	Verify(checksum string) bool
	ResolveEntrypoint(entrypoint string) (string, error)
	ResolveDir(dir string) (string, error)
}

func NewRootNode(
	entrypoint string,
	dir string,
	opts ...NodeOption,
) (Node, error) {
	dir = fsext.DefaultDir(entrypoint, dir)
	if entrypoint == "-" {
		return NewStdinNode(dir)
	}
	return NewNode(entrypoint, dir, opts...)
}

func NewNode(
	entrypoint string,
	dir string,
	opts ...NodeOption,
) (Node, error) {
	if hasURLScheme(entrypoint) {
		return nil, errors.New("rite: remote Ritefiles are not supported — check Ritefiles in to your repo to keep task execution idempotent")
	}
	return NewFileNode(entrypoint, dir, opts...)
}

func hasURLScheme(s string) bool {
	return strings.Contains(s, "://")
}
