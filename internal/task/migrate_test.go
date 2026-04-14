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

func TestMigrate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "Taskfile.yml")
	if err := os.WriteFile(src, []byte(`# yaml-language-server: $schema=https://taskfile.dev/schema.json
version: '3'
vars:
  GLOBAL: from-global
  GITHUB_TOKEN: secret
env:
  NODE_ENV: production
includes:
  lib:
    taskfile: ./lib/Taskfile.yml
  tools: Taskfile-tools.yml
tasks:
  build:
    vars:
      GLOBAL: from-task
    env:
      NODE_ENV: development
    cmds:
      - echo hi
`), 0o644); err != nil {
		t.Fatal(err)
	}

	var warn bytes.Buffer
	dst, err := task.Migrate(src, &warn)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := filepath.Base(dst), "Ritefile.yml"; got != want {
		t.Errorf("dst basename = %q, want %q", got, want)
	}

	out, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	// Include paths in the `includes:` block must be rewritten.
	wantFragments := []string{
		"taskfile: ./lib/Ritefile.yml",
		"tools: Ritefile-tools.yml",
	}
	for _, frag := range wantFragments {
		if !strings.Contains(got, frag) {
			t.Errorf("output missing %q\nGOT:\n%s", frag, got)
		}
	}

	// `Taskfile` elsewhere (schema comment here) is left alone — docs
	// cleanup is the user's call after they see the SCHEMA-URL warning.
	if !strings.Contains(got, "taskfile.dev/schema.json") {
		t.Error("schema URL was rewritten — should be left verbatim so the warning lands on real text")
	}

	// Check warnings.
	warnS := warn.String()
	wantWarns := []string{
		"OVERRIDE-VAR", // GLOBAL shadowed
		"OVERRIDE-ENV", // NODE_ENV shadowed
		"SECRET-VAR",   // GITHUB_TOKEN name pattern
		"SCHEMA-URL",   // taskfile.dev pointer
	}
	for _, w := range wantWarns {
		if !strings.Contains(warnS, w) {
			t.Errorf("warnings missing %q\nGOT:\n%s", w, warnS)
		}
	}
}

// TestMigrateRewritesSpecialVars locks in the rite-named special-var alias
// rewrite: occurrences of .TASK and .TASK_DIR inside `{{ … }}` become
// .RITE_NAME and .RITE_TASK_DIR after migration. Both names work at
// runtime — this is a readability nudge to steer migrated Ritefiles
// toward the SPEC-preferred surface. References outside template
// expressions (prose, comments mentioning TASK_DIR) are left alone.
func TestMigrateRewritesSpecialVars(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "Taskfile.yml")
	input := `version: '3'
tasks:
  print-task:
    cmds:
      - echo {{.TASK}}
      - echo "running {{ .TASK }} in {{.TASK_DIR}}"
  other:
    # .TASK in a comment is left alone — not inside a template expr.
    vars:
      MY_TASK_DIR: static   # substring MY_TASK_DIR must not be rewritten
    cmds:
      - echo {{.MY_TASK_DIR}}
`
	if err := os.WriteFile(src, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	dst, err := task.Migrate(src, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	wantFrags := []string{
		"echo {{.RITE_NAME}}",
		"running {{ .RITE_NAME }} in {{.RITE_TASK_DIR}}",
		"{{.MY_TASK_DIR}}",     // user var stays untouched
		"# .TASK in a comment", // non-template occurrence stays untouched
	}
	for _, w := range wantFrags {
		if !strings.Contains(got, w) {
			t.Errorf("output missing %q\nGOT:\n%s", w, got)
		}
	}

	unwantFrags := []string{
		".RITE_NAME_DIR",  // sanity: don't half-rename .TASK_DIR
		".RITE_TASK_DIR_", // no cascading
	}
	for _, w := range unwantFrags {
		if strings.Contains(got, w) {
			t.Errorf("output unexpectedly contains %q\nGOT:\n%s", w, got)
		}
	}
}

// TestMigrateRewritesLegacySpecialVars locks in the rewrites for the
// four special vars that were initially missed when `rite migrate`
// shipped: .TASKFILE, .TASKFILE_DIR, .ROOT_TASKFILE, .TASK_VERSION.
// Before #36 these rendered as the empty string because neither a
// migrate rewrite nor a runtime alias existed. Both halves are locked
// in: this test covers the migrate rewrite; the runtime alias is
// covered by TestLegacySpecialVarAliasesResolve below.
func TestMigrateRewritesLegacySpecialVars(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "Taskfile.yml")
	input := `version: '3'
tasks:
  banner:
    cmds:
      - echo "{{.TASKFILE}} lives in {{.TASKFILE_DIR}}"
      - echo "root is {{.ROOT_TASKFILE}}"
      - echo "version {{.TASK_VERSION}}"
  other:
    # .TASKFILE in a comment is left alone.
    vars:
      MY_TASKFILE: static
    cmds:
      - echo {{.MY_TASKFILE}}
`
	if err := os.WriteFile(src, []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	dst, err := task.Migrate(src, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	wantFrags := []string{
		`{{.RITEFILE}} lives in {{.RITEFILE_DIR}}`,
		`root is {{.ROOT_RITEFILE}}`,
		`version {{.RITE_VERSION}}`,
		`{{.MY_TASKFILE}}`,         // user var stays untouched
		`# .TASKFILE in a comment`, // non-template occurrence stays untouched
	}
	for _, w := range wantFrags {
		if !strings.Contains(got, w) {
			t.Errorf("output missing %q\nGOT:\n%s", w, got)
		}
	}

	unwantFrags := []string{
		".TASKFILE}",      // old name should not survive inside a template
		".TASKFILE_DIR}",  // ditto
		".ROOT_TASKFILE}", // ditto
		".TASK_VERSION}",  // ditto
		".RITEFILE_DIR_",  // no cascading
		".ROOT_RITEFILE_", // no cascading
	}
	for _, w := range unwantFrags {
		if strings.Contains(got, w) {
			t.Errorf("output unexpectedly contains %q\nGOT:\n%s", w, got)
		}
	}
}

// TestMigrateRewritesAdjacentSpecialVars covers the case where two
// .TASK / .TASK_DIR refs appear inside the same `{{ … }}` expression,
// with or without a separator between them. A single pass of the
// bracket-class regex consumes the shared separator and misses match
// #2, so the rewriter runs to a fixed point. See issue #10.
func TestMigrateRewritesAdjacentSpecialVars(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, cmd string
		want      []string
	}{
		{
			name: "two_task_dir_space_separated",
			cmd:  `echo {{.TASK_DIR .TASK_DIR}}`,
			want: []string{"{{.RITE_TASK_DIR .RITE_TASK_DIR}}"},
		},
		{
			name: "two_task_dir_via_printf",
			cmd:  `echo {{printf "%s/%s" .TASK_DIR .TASK_DIR}}`,
			want: []string{`{{printf "%s/%s" .RITE_TASK_DIR .RITE_TASK_DIR}}`},
		},
		{
			name: "two_task_space_separated",
			cmd:  `echo {{.TASK .TASK}}`,
			want: []string{"{{.RITE_NAME .RITE_NAME}}"},
		},
		{
			name: "three_task_dir_space_separated",
			cmd:  `echo {{.TASK_DIR .TASK_DIR .TASK_DIR}}`,
			want: []string{"{{.RITE_TASK_DIR .RITE_TASK_DIR .RITE_TASK_DIR}}"},
		},
		{
			name: "mixed_task_and_task_dir",
			cmd:  `echo {{printf "%s/%s" .TASK .TASK_DIR}}`,
			want: []string{`{{printf "%s/%s" .RITE_NAME .RITE_TASK_DIR}}`},
		},
		{
			name: "separate_expressions_still_work",
			cmd:  `echo {{.TASK}}{{.TASK_DIR}}`,
			want: []string{"{{.RITE_NAME}}{{.RITE_TASK_DIR}}"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			src := filepath.Join(dir, "Taskfile.yml")
			input := "version: '3'\ntasks:\n  t:\n    cmds:\n      - " + c.cmd + "\n"
			if err := os.WriteFile(src, []byte(input), 0o644); err != nil {
				t.Fatal(err)
			}
			dst, err := task.Migrate(src, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			out, err := os.ReadFile(dst)
			if err != nil {
				t.Fatal(err)
			}
			got := string(out)
			for _, w := range c.want {
				if !strings.Contains(got, w) {
					t.Errorf("output missing %q\nGOT:\n%s", w, got)
				}
			}
			if strings.Contains(got, ".TASK_DIR") || strings.Contains(got, ".TASK ") || strings.Contains(got, ".TASK}") {
				t.Errorf("output still contains un-rewritten ref\nGOT:\n%s", got)
			}
		})
	}
}

// TestLegacySpecialVarAliasesResolve runs a Ritefile that references the
// go-task special var names (.TASKFILE, .TASKFILE_DIR, .ROOT_TASKFILE,
// .TASK_VERSION) and asserts each one renders a non-empty, sensible
// value. This is the runtime half of the #36 fix: even for Ritefiles
// that never pass through `rite migrate` (hand-authored, or authored
// before the rewriter learned these names), the compat aliases have
// to keep producing the expected values instead of the empty string.
func TestLegacySpecialVarAliasesResolve(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ritefilePath := filepath.Join(dir, "Ritefile.yml")
	body := `version: '3'
tasks:
  legacy:
    cmds:
      - echo "TASKFILE={{.TASKFILE}}"
      - echo "TASKFILE_DIR={{.TASKFILE_DIR}}"
      - echo "ROOT_TASKFILE={{.ROOT_TASKFILE}}"
      - echo "TASK_VERSION={{.TASK_VERSION}}"
`
	require.NoError(t, os.WriteFile(ritefilePath, []byte(body), 0o644))

	var stdout bytes.Buffer
	e := task.NewExecutor(
		task.WithDir(dir),
		task.WithEntrypoint(ritefilePath),
		task.WithStdout(&stdout),
		task.WithStderr(io.Discard),
		task.WithSilent(true),
	)
	require.NoError(t, e.Setup(), "Setup")
	require.NoError(t, e.Run(t.Context(), &task.Call{Task: "legacy"}), "Run")

	got := stdout.String()
	// The exact on-disk path representation uses forward slashes (the
	// compiler calls filepath.ToSlash on these values for cross-platform
	// consistency), so compare against a normalized expected string.
	wantLines := []string{
		"TASKFILE=" + filepath.ToSlash(ritefilePath),
		"TASKFILE_DIR=" + filepath.ToSlash(dir),
		"ROOT_TASKFILE=" + filepath.ToSlash(ritefilePath),
	}
	for _, w := range wantLines {
		if !strings.Contains(got, w) {
			t.Errorf("output missing %q\nGOT:\n%s", w, got)
		}
	}
	// TASK_VERSION comes from internal/version; we don't assert the exact
	// string (it's "3.49.1" in dev builds, set via ldflags in releases) —
	// only that it's present and non-empty.
	if strings.Contains(got, "TASK_VERSION=\n") || !strings.Contains(got, "TASK_VERSION=") {
		t.Errorf("TASK_VERSION rendered empty\nGOT:\n%s", got)
	}
}

func TestMigrateRitefilePath(t *testing.T) {
	t.Parallel()
	// filepath.Join on Windows normalizes to backslash separators, so express
	// expected paths via filepath.Join rather than hand-writing Unix literals.
	// The helper itself uses filepath.Split/Join so it's platform-correct;
	// this just makes the test's expected values match the runtime idiom.
	join := func(parts ...string) string { return filepath.Join(parts...) }
	cases := []struct {
		src, want string
	}{
		{join("a", "b", "Taskfile.yml"), join("a", "b", "Ritefile.yml")},
		{join("a", "b", "Taskfile.yaml"), join("a", "b", "Ritefile.yaml")},
		{join("a", "b", "Taskfile.dist.yml"), join("a", "b", "Ritefile.dist.yml")},
		{join("a", "b", "Taskfile-inc.yml"), join("a", "b", "Ritefile-inc.yml")},
		{join("a", "b", "taskfile.yml"), join("a", "b", "ritefile.yml")},
		{join("a", "b", "something.yml"), join("a", "b", "Ritefile.yml")},
	}
	for _, c := range cases {
		got := task.RitefilePathForTest(c.src)
		if got != c.want {
			t.Errorf("RitefilePath(%q) = %q, want %q", c.src, got, c.want)
		}
	}
}
