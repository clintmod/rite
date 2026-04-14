package taskfile

import (
	"io"
	"os"
	"path/filepath"

	"github.com/clintmod/rite/errors"
	"github.com/clintmod/rite/internal/execext"
	"github.com/clintmod/rite/internal/filepathext"
	"github.com/clintmod/rite/internal/fsext"
)

// A FileNode is a node that reads a taskfile from the local filesystem.
type FileNode struct {
	*baseNode
	entrypoint string
}

func NewFileNode(entrypoint, dir string, opts ...NodeOption) (*FileNode, error) {
	// Find the entrypoint file
	resolvedEntrypoint, err := fsext.Search(entrypoint, dir, DefaultRitefiles)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if entrypoint == "" {
				return nil, errors.RitefileNotFoundError{URI: entrypoint, Walk: true}
			} else {
				return nil, errors.RitefileNotFoundError{URI: entrypoint, Walk: false}
			}
		} else if errors.Is(err, os.ErrPermission) {
			return nil, errors.RitefileNotFoundError{URI: entrypoint, Walk: true, OwnerChange: true}
		}
		return nil, err
	}

	// Resolve the directory
	resolvedDir, err := fsext.ResolveDir(entrypoint, resolvedEntrypoint, dir)
	if err != nil {
		return nil, err
	}

	return &FileNode{
		baseNode:   NewBaseNode(resolvedDir, opts...),
		entrypoint: resolvedEntrypoint,
	}, nil
}

func (node *FileNode) Location() string {
	return node.entrypoint
}

func (node *FileNode) Read() ([]byte, error) {
	f, err := os.Open(node.Location())
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func (node *FileNode) ResolveEntrypoint(entrypoint string) (string, error) {
	path, err := execext.ExpandLiteral(entrypoint)
	if err != nil {
		return "", err
	}

	if filepathext.IsAbs(path) {
		return path, nil
	}

	// NOTE: Uses the directory of the entrypoint (Ritefile), not the current working directory
	// This means that files are included relative to one another
	entrypointDir := filepath.Dir(node.entrypoint)
	return filepathext.SmartJoin(entrypointDir, path), nil
}

func (node *FileNode) ResolveDir(dir string) (string, error) {
	path, err := execext.ExpandLiteral(dir)
	if err != nil {
		return "", err
	}

	if filepathext.IsAbs(path) {
		return path, nil
	}

	// NOTE: Uses the directory of the entrypoint (Ritefile), not the current working directory
	// This means that files are included relative to one another
	entrypointDir := filepath.Dir(node.entrypoint)
	return filepathext.SmartJoin(entrypointDir, path), nil
}
