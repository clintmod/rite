package task_test

import (
	"bytes"
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
