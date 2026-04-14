package task

import (
	_ "embed"
	"os"

	"github.com/clintmod/rite/errors"
	"github.com/clintmod/rite/internal/filepathext"
	"github.com/clintmod/rite/taskfile"
)

const defaultFilename = "Ritefile.yml"

//go:embed templates/default.yml
var DefaultRitefile string

// InitRitefile creates a new Ritefile at path.
//
// path can be either a file path or a directory path.
// If path is a directory, path/Ritefile.yml will be created.
//
// The final file path is always returned and may be different from the input path.
func InitRitefile(path string) (string, error) {
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		return path, errors.RitefileAlreadyExistsError{}
	}

	if info != nil && info.IsDir() {
		// path was a directory, check if there is a Ritefile already
		if hasDefaultRitefile(path) {
			return path, errors.RitefileAlreadyExistsError{}
		}
		path = filepathext.SmartJoin(path, defaultFilename)
	}

	if err := os.WriteFile(path, []byte(DefaultRitefile), 0o644); err != nil {
		return path, err
	}
	return path, nil
}

func hasDefaultRitefile(dir string) bool {
	for _, name := range taskfile.DefaultRitefiles {
		if _, err := os.Stat(filepathext.SmartJoin(dir, name)); err == nil {
			return true
		}
	}
	return false
}
