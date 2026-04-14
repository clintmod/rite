package riterc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/clintmod/rite/riterc/ast"
)

const (
	xdgConfigYAML = `
experiments:
  FOO: 1
  BAR: 1
  BAZ: 1
`

	homeConfigYAML = `
experiments:
  FOO: 2
  BAR: 2
`

	localConfigYAML = `
experiments:
  FOO: 3
`
)

func setupDirs(t *testing.T) (string, string, string) {
	t.Helper()

	xdgConfigDir := t.TempDir()
	xdgTaskConfigDir := filepath.Join(xdgConfigDir, "rite")
	require.NoError(t, os.Mkdir(xdgTaskConfigDir, 0o755))

	homeDir := t.TempDir()

	localDir := filepath.Join(homeDir, "local")
	require.NoError(t, os.Mkdir(localDir, 0o755))

	t.Setenv("XDG_CONFIG_HOME", xdgConfigDir)
	t.Setenv("HOME", homeDir)

	return xdgTaskConfigDir, homeDir, localDir
}

func writeFile(t *testing.T, dir, filename, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644)
	assert.NoError(t, err)
}

func TestGetConfig_NoConfigFiles(t *testing.T) { //nolint:paralleltest // cannot run in parallel
	_, _, localDir := setupDirs(t)

	cfg, err := GetConfig(localDir)
	assert.NoError(t, err)
	assert.Nil(t, cfg)
}

func TestGetConfig_OnlyXDG(t *testing.T) { //nolint:paralleltest // cannot run in parallel
	xdgDir, _, localDir := setupDirs(t)

	writeFile(t, xdgDir, "riterc.yml", xdgConfigYAML)

	cfg, err := GetConfig(localDir)
	assert.NoError(t, err)
	assert.Equal(t, &ast.TaskRC{
		Version: nil,
		Experiments: map[string]int{
			"FOO": 1,
			"BAR": 1,
			"BAZ": 1,
		},
	}, cfg)
}

func TestGetConfig_OnlyHome(t *testing.T) { //nolint:paralleltest // cannot run in parallel
	_, homeDir, localDir := setupDirs(t)

	writeFile(t, homeDir, ".riterc.yml", homeConfigYAML)

	cfg, err := GetConfig(localDir)
	assert.NoError(t, err)
	assert.Equal(t, &ast.TaskRC{
		Version: nil,
		Experiments: map[string]int{
			"FOO": 2,
			"BAR": 2,
		},
	}, cfg)
}

func TestGetConfig_OnlyLocal(t *testing.T) { //nolint:paralleltest // cannot run in parallel
	_, _, localDir := setupDirs(t)

	writeFile(t, localDir, ".riterc.yml", localConfigYAML)

	cfg, err := GetConfig(localDir)
	assert.NoError(t, err)
	assert.Equal(t, &ast.TaskRC{
		Version: nil,
		Experiments: map[string]int{
			"FOO": 3,
		},
	}, cfg)
}

func TestGetConfig_All(t *testing.T) { //nolint:paralleltest // cannot run in parallel
	xdgConfigDir, homeDir, localDir := setupDirs(t)

	// Write local config
	writeFile(t, localDir, ".riterc.yml", localConfigYAML)

	// Write home config
	writeFile(t, homeDir, ".riterc.yml", homeConfigYAML)

	// Write XDG config
	writeFile(t, xdgConfigDir, "riterc.yml", xdgConfigYAML)

	cfg, err := GetConfig(localDir)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, &ast.TaskRC{
		Version: nil,
		Experiments: map[string]int{
			"FOO": 3,
			"BAR": 2,
			"BAZ": 1,
		},
	}, cfg)
}

// TestGetConfig_CoexistenceWithTaskfile locks in the SPEC "On-disk Paths"
// guarantee: rite MUST only read .riterc.yml, never .taskrc.yml. A project
// checking in both files alongside each other exercises both tools; rite
// must ignore go-task's config entirely. Regressions here would quietly
// merge a user's go-task settings into rite and vice-versa — the whole
// point of the fork is to keep those contracts separate.
func TestGetConfig_CoexistenceWithTaskfile(t *testing.T) { //nolint:paralleltest // uses t.Setenv
	_, _, localDir := setupDirs(t)

	// A .taskrc.yml with distinctive content rite must NOT read.
	taskrcYAML := `
experiments:
  SHOULD_NOT_APPEAR: 99
`
	writeFile(t, localDir, ".taskrc.yml", taskrcYAML)

	// With only .taskrc.yml present, GetConfig should find no rite config.
	cfg, err := GetConfig(localDir)
	assert.NoError(t, err)
	assert.Nil(t, cfg, "rite read .taskrc.yml; coexistence is broken")

	// Now add a .riterc.yml beside it — rite reads its own, still ignores
	// the go-task file.
	writeFile(t, localDir, ".riterc.yml", localConfigYAML)

	cfg, err = GetConfig(localDir)
	assert.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, &ast.TaskRC{
		Version: nil,
		Experiments: map[string]int{
			"FOO": 3,
		},
	}, cfg, "rite config must reflect .riterc.yml only, never .taskrc.yml")
}
