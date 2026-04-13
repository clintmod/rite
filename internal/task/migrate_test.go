package task_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
