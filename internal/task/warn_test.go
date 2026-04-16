package task_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	task "github.com/clintmod/rite/internal/task"
)

// TestDetectRedundantSelfRef covers the regex / key-match heuristic in
// detectRedundantSelfRef. We route through the public-ish Setup entry point
// via TestSetupWarnsOnRedundantSelfRef; here we re-test the same logic
// indirectly by building minimal Ritefiles for each table case and
// asserting the stderr contract, so the unit-level logic is locked in
// without exporting the helper itself.
func TestDetectRedundantSelfRef(t *testing.T) {
	t.Parallel()
	type tc struct {
		name    string
		key     string
		value   string
		wantOK  bool
		wantDef string
	}
	cases := []tc{
		// positives
		{"bare", "VAR", "${VAR}", true, ""},
		{"with_default", "VAR", "${VAR:-foo}", true, "foo"},
		{"vm_version_like", "VM_VERSION", "${VM_VERSION:-1.0.1}", true, "1.0.1"},
		{"whitespace_tolerated", "VAR", "  ${VAR:-x}  ", true, "x"},
		{"empty_default", "VAR", "${VAR:-}", true, ""},
		// negatives
		{"different_name_bare", "VAR", "${OTHER}", false, ""},
		{"different_name_default", "VAR", "${OTHER:-x}", false, ""},
		{"prefix", "VAR", "prefix${VAR}", false, ""},
		{"suffix", "VAR", "${VAR}suffix", false, ""},
		{"nested_default", "VAR", "${VAR:-${NESTED}}", false, ""},
		{"literal", "VAR", "literal", false, ""},
		{"empty", "VAR", "", false, ""},
		{"case_sensitive", "var", "${VAR}", false, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			// Build a tempdir Ritefile with just this var so we can observe
			// the stderr output Setup produces. This keeps the helper
			// unexported while still exercising every branch.
			tmp := t.TempDir()
			ritefile := "version: '3'\nvars:\n  " + c.key + ": '" + escapeYAML(c.value) + "'\ntasks:\n  default:\n    cmds:\n      - echo hi\n"
			require.NoError(t, os.WriteFile(filepath.Join(tmp, "Ritefile"), []byte(ritefile), 0o644))

			var stderr bytes.Buffer
			e := task.NewExecutor(
				task.WithDir(tmp),
				task.WithStdout(io.Discard),
				task.WithStderr(&stderr),
			)
			require.NoError(t, e.Setup())
			got := stderr.String()
			if c.wantOK {
				require.Contains(t, got, "is redundant", "expected warning for %q: %q", c.key, c.value)
				require.Contains(t, got, c.key, "expected key %q in warning, got %q", c.key, got)
				if c.wantDef != "" {
					require.Contains(t, got, c.wantDef, "expected default %q in warning, got %q", c.wantDef, got)
				}
			} else {
				require.NotContains(t, got, "is redundant", "unexpected warning for %q: %q: %s", c.key, c.value, got)
			}
		})
	}
}

// escapeYAML handles the handful of tricky values in the table above — the
// only real problem is single quotes inside the value, which YAML
// single-quoted scalars escape by doubling.
func escapeYAML(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// TestSetupWarnsOnRedundantSelfRef is the end-to-end smoke: a Ritefile with
// the `VM_VERSION: '${VM_VERSION:-1.0.1}'` pattern emits the expected
// warning at Setup() time, and a clean Ritefile emits none.
func TestSetupWarnsOnRedundantSelfRef(t *testing.T) {
	t.Parallel()

	t.Run("warns_when_self_ref_present", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		ritefile := `version: '3'
vars:
  VM_VERSION: '${VM_VERSION:-1.0.1}'
tasks:
  default:
    cmds:
      - echo hi
`
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "Ritefile"), []byte(ritefile), 0o644))

		var stderr bytes.Buffer
		e := task.NewExecutor(
			task.WithDir(tmp),
			task.WithStdout(io.Discard),
			task.WithStderr(&stderr),
		)
		require.NoError(t, e.Setup())
		got := stderr.String()
		require.Contains(t, got, "is redundant")
		require.Contains(t, got, "VM_VERSION")
	})

	t.Run("silent_on_clean_ritefile", func(t *testing.T) {
		t.Parallel()
		tmp := t.TempDir()
		ritefile := `version: '3'
vars:
  VM_VERSION: 1.0.1
tasks:
  default:
    cmds:
      - echo hi
`
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "Ritefile"), []byte(ritefile), 0o644))

		var stderr bytes.Buffer
		e := task.NewExecutor(
			task.WithDir(tmp),
			task.WithStdout(io.Discard),
			task.WithStderr(&stderr),
		)
		require.NoError(t, e.Setup())
		require.NotContains(t, stderr.String(), "is redundant")
	})
}
