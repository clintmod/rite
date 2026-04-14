package task_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/clintmod/rite/errors"
	task "github.com/clintmod/rite/internal/task"
)

func TestValidate_Valid(t *testing.T) {
	t.Parallel()
	e := task.NewExecutor(
		task.WithDir("testdata/validate/valid"),
		task.WithVersionCheck(true),
	)
	require.NoError(t, e.Validate())
	require.NotNil(t, e.Ritefile, "Ritefile should be populated on success")
}

func TestValidate_NotFound(t *testing.T) {
	t.Parallel()
	// Point at an explicit entrypoint that definitely doesn't exist. A
	// relative missing-dir path (the previous approach) behaves differently
	// across platforms: on Windows CI, `os.Stat` on a non-existent relative
	// path doesn't cleanly return `os.ErrNotExist` the way it does on unix,
	// so the root-walk logic doesn't always map to `RitefileNotFoundError`.
	// An absolute tempdir path sidesteps the ambiguity.
	dir := t.TempDir()
	e := task.NewExecutor(
		task.WithDir(dir),
		task.WithEntrypoint(filepath.Join(dir, "Ritefile.yml")),
		task.WithVersionCheck(true),
	)
	err := e.Validate()
	require.Error(t, err)
	var nf errors.RitefileNotFoundError
	assert.ErrorAs(t, err, &nf)
}

func TestValidate_YAMLError(t *testing.T) {
	t.Parallel()
	e := task.NewExecutor(
		task.WithDir("testdata/validate/yaml_error"),
		task.WithVersionCheck(true),
	)
	err := e.Validate()
	require.Error(t, err)
	// Pure YAML parse failures surface as RitefileInvalidError (exit code 104).
	// Shape-mismatch decode failures (wrong AST type) surface as
	// RitefileDecodeError (exit code 102). Either is an acceptable validate
	// outcome — what matters is that it's a typed TaskError that routes through
	// the exit-code dispatcher instead of escaping as CodeUnknown.
	te, ok := err.(errors.TaskError)
	require.True(t, ok, "validate error should implement errors.TaskError for exit-code routing")
	assert.Contains(t,
		[]int{errors.CodeRitefileInvalid, errors.CodeRitefileDecode},
		te.Code(),
	)
}

func TestValidate_VersionTooLow(t *testing.T) {
	t.Parallel()
	e := task.NewExecutor(
		task.WithDir("testdata/version/v1"),
		task.WithVersionCheck(true),
	)
	err := e.Validate()
	require.Error(t, err)
	var ve *errors.RitefileVersionCheckError
	assert.ErrorAs(t, err, &ve)
}

func TestValidate_VersionTooHigh(t *testing.T) {
	t.Parallel()
	e := task.NewExecutor(
		task.WithDir("testdata/version/v4"),
		task.WithVersionCheck(true),
	)
	err := e.Validate()
	require.Error(t, err)
	var ve *errors.RitefileVersionCheckError
	assert.ErrorAs(t, err, &ve)
}

// A Ritefile whose `sh:` var would fail if executed still validates,
// because Validate() skips dotenv/compiler and therefore never evaluates
// dynamic vars. This is the whole point of --validate for CI lint stages.
func TestValidate_SkipsShVarEvaluation(t *testing.T) {
	t.Parallel()
	e := task.NewExecutor(
		task.WithDir("testdata/validate/sh_var"),
		task.WithVersionCheck(true),
	)
	require.NoError(t, e.Validate())
}

func TestValidate_CyclicInclude(t *testing.T) {
	t.Parallel()
	e := task.NewExecutor(
		task.WithDir("testdata/includes_cycle"),
		task.WithVersionCheck(true),
	)
	err := e.Validate()
	require.Error(t, err)
}
