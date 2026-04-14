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

// TestMigrateWalksIncludes locks in the #41 fix: `rite migrate` has to walk
// the entrypoint's `includes:` block and migrate each local file too,
// otherwise the rewritten include paths point at files that don't exist on
// disk and the migrated Ritefile fails to load.
func TestMigrateWalksIncludes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".taskfiles"), 0o755))

	// Entrypoint with mixed include shapes: scalar shortcut, long form with
	// `taskfile:` key, and a URL (must be skipped).
	entry := filepath.Join(dir, "Taskfile.yml")
	require.NoError(t, os.WriteFile(entry, []byte(`version: '3'
includes:
  short: .taskfiles/short.Taskfile.yml
  long:
    taskfile: .taskfiles/long.Taskfile.yml
    optional: true
  remote:
    taskfile: https://example.com/remote.yml
tasks:
  hi:
    cmds:
      - echo hi
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".taskfiles", "short.Taskfile.yml"),
		[]byte(`version: '3'
tasks:
  from-short:
    cmds: ['echo short']
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".taskfiles", "long.Taskfile.yml"),
		[]byte(`version: '3'
tasks:
  from-long:
    cmds: ['echo long']
`), 0o644))

	var warn bytes.Buffer
	dst, err := task.Migrate(entry, &warn)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "Ritefile.yml"), dst)

	// Every included local file must now exist as a sibling Ritefile.yml.
	for _, want := range []string{
		filepath.Join(dir, "Ritefile.yml"),
		filepath.Join(dir, ".taskfiles", "short.Ritefile.yml"),
		filepath.Join(dir, ".taskfiles", "long.Ritefile.yml"),
	} {
		if _, err := os.Stat(want); err != nil {
			t.Errorf("expected migrated file at %s: %v", want, err)
		}
	}
	// Original Taskfile.yml must be left in place — migration is non-destructive.
	for _, want := range []string{
		entry,
		filepath.Join(dir, ".taskfiles", "short.Taskfile.yml"),
		filepath.Join(dir, ".taskfiles", "long.Taskfile.yml"),
	} {
		if _, err := os.Stat(want); err != nil {
			t.Errorf("expected original Taskfile preserved at %s: %v", want, err)
		}
	}
	// URL include must be reported, not migrated.
	warnS := warn.String()
	require.Contains(t, warnS, "skipping remote include")
	require.Contains(t, warnS, "https://example.com/remote.yml")
	// Included-file writes are logged for the user.
	require.Contains(t, warnS, "wrote ")
	require.Contains(t, warnS, "short.Ritefile.yml")
	require.Contains(t, warnS, "long.Ritefile.yml")
}

// TestMigrateWalksIncludesMissingNonOptional surfaces the difference in
// behavior between required and optional includes pointing at missing files:
// a missing required include earns a warning; a missing optional one is
// silent. In both cases, migration of the entrypoint still succeeds.
func TestMigrateWalksIncludesMissingNonOptional(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	entry := filepath.Join(dir, "Taskfile.yml")
	require.NoError(t, os.WriteFile(entry, []byte(`version: '3'
includes:
  missing: .taskfiles/gone.Taskfile.yml
  optmissing:
    taskfile: .taskfiles/alsogone.Taskfile.yml
    optional: true
tasks: {hi: {cmds: ['echo hi']}}
`), 0o644))

	var warn bytes.Buffer
	dst, err := task.Migrate(entry, &warn)
	require.NoError(t, err)
	require.FileExists(t, dst)
	warnS := warn.String()
	require.Contains(t, warnS, `include "missing"`)
	require.NotContains(t, warnS, `include "optmissing"`)
}

// TestMigrateWalksIncludesCycle confirms that a cycle in the includes graph
// (A → B → A) does not cause infinite recursion. Each file is migrated
// exactly once.
func TestMigrateWalksIncludesCycle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := filepath.Join(dir, "Taskfile.yml")
	b := filepath.Join(dir, "Taskfile-b.yml")
	require.NoError(t, os.WriteFile(a, []byte(`version: '3'
includes:
  b: Taskfile-b.yml
tasks: {hi: {cmds: ['echo hi']}}
`), 0o644))
	require.NoError(t, os.WriteFile(b, []byte(`version: '3'
includes:
  a: Taskfile.yml
tasks: {bye: {cmds: ['echo bye']}}
`), 0o644))

	_, err := task.Migrate(a, io.Discard)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(dir, "Ritefile.yml"))
	require.FileExists(t, filepath.Join(dir, "Ritefile-b.yml"))
}

// TestMigrateDryRun confirms no files are written in dry-run mode, but
// warnings + "would write" announcements still land in the warn writer for
// every file (root + includes).
func TestMigrateDryRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	entry := filepath.Join(dir, "Taskfile.yml")
	require.NoError(t, os.WriteFile(entry, []byte(`version: '3'
includes:
  s: sub/Taskfile.yml
tasks: {hi: {cmds: ['echo hi']}}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "Taskfile.yml"),
		[]byte("version: '3'\ntasks: {x: {cmds: ['echo x']}}\n"), 0o644))

	var warn bytes.Buffer
	_, err := task.Migrate(entry, &warn, task.WithDryRun(true))
	require.NoError(t, err)

	// Nothing on disk.
	for _, p := range []string{
		filepath.Join(dir, "Ritefile.yml"),
		filepath.Join(dir, "sub", "Ritefile.yml"),
	} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expected %s to not exist in dry-run, got err=%v", p, err)
		}
	}
	// Nested file still gets a `would write` announcement; the entrypoint
	// is suppressed because the CLI announces it separately.
	warnS := warn.String()
	require.Contains(t, warnS, "would write ")
	require.Contains(t, warnS, filepath.Join(dir, "sub", "Ritefile.yml"))
}

// TestMigrateSiblingIncludesDontClobber is the #76 regression test: multiple
// sibling includes whose source basenames don't contain "Taskfile" (e.g.
// `ci/a.yml`, `ci/b.yml`, `ci/c.yml`) used to all migrate to the same
// destination filename (`ci/Ritefile.yml`), clobbering each other. Now each
// gets a per-source destination (`ci/a.Ritefile.yml`, `ci/b.Ritefile.yml`,
// `ci/c.Ritefile.yml`) and the root's `includes:` block is rewritten to
// point at the new paths.
func TestMigrateSiblingIncludesDontClobber(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ci"), 0o755))

	entry := filepath.Join(dir, "Taskfile.yml")
	require.NoError(t, os.WriteFile(entry, []byte(`version: '3'
includes:
  a: ci/a.yml
  b: ci/b.yml
  c: ci/c.yml
tasks:
  default:
    cmds: ['echo hi']
`), 0o644))
	for _, name := range []string{"a", "b", "c"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "ci", name+".yml"),
			[]byte("version: '3'\ntasks:\n  from-"+name+":\n    cmds: ['echo "+name+"']\n"),
			0o644))
	}

	var warn bytes.Buffer
	dst, err := task.Migrate(entry, &warn)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "Ritefile.yml"), dst)

	// Each include must land at a distinct destination — not clobbered.
	for _, name := range []string{"a", "b", "c"} {
		p := filepath.Join(dir, "ci", name+".Ritefile.yml")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected migrated include at %s: %v", p, err)
			continue
		}
		body, err := os.ReadFile(p)
		require.NoError(t, err)
		if !strings.Contains(string(body), "from-"+name) {
			t.Errorf("%s does not contain from-%s — likely clobbered\nGOT:\n%s", p, name, body)
		}
	}

	// The old collapsed filename must not exist.
	if _, err := os.Stat(filepath.Join(dir, "ci", "Ritefile.yml")); !os.IsNotExist(err) {
		t.Errorf("unexpected legacy clobber path ci/Ritefile.yml (err=%v)", err)
	}

	// Root's includes: block must be rewritten to the new per-source paths.
	rootBody, err := os.ReadFile(dst)
	require.NoError(t, err)
	for _, want := range []string{
		"a: ci/a.Ritefile.yml",
		"b: ci/b.Ritefile.yml",
		"c: ci/c.Ritefile.yml",
	} {
		if !strings.Contains(string(rootBody), want) {
			t.Errorf("root Ritefile.yml missing rewritten include %q\nGOT:\n%s", want, rootBody)
		}
	}

	// The original Taskfiles must be preserved — migration is non-destructive.
	for _, name := range []string{"a", "b", "c"} {
		p := filepath.Join(dir, "ci", name+".yml")
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected original %s preserved: %v", p, err)
		}
	}
}

// TestMigrateIncludedRitefilePath is the unit-level counterpart to
// TestMigrateSiblingIncludesDontClobber: it exercises the per-source
// destination-name mapping directly. The entrypoint case stays on
// ritefilePath (canonical Ritefile.yml); nested includes go through
// includedRitefilePath, which must preserve stems.
func TestMigrateIncludedRitefilePath(t *testing.T) {
	t.Parallel()
	join := func(parts ...string) string { return filepath.Join(parts...) }
	cases := []struct {
		src, want string
	}{
		{join("ci", "a.yml"), join("ci", "a.Ritefile.yml")},
		{join("ci", "android.yaml"), join("ci", "android.Ritefile.yaml")},
		{join("ci", "android.Taskfile.yml"), join("ci", "android.Ritefile.yml")},
		{join("ci", "Taskfile.yml"), join("ci", "Ritefile.yml")},
		{join("ci", "taskfile.yml"), join("ci", "ritefile.yml")},
		{join("ci", "noext"), join("ci", "noext.Ritefile.yml")},
	}
	for _, c := range cases {
		got := task.IncludedRitefilePathForTest(c.src)
		if got != c.want {
			t.Errorf("IncludedRitefilePath(%q) = %q, want %q", c.src, got, c.want)
		}
	}
}

// TestMigrateRewritesIncludeForNestedFiles covers the scenario where a
// migrated included file itself has includes pointing further down. Every
// level's includes: block has to be rewritten to the new per-source paths,
// not just the root's.
func TestMigrateRewritesIncludeForNestedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ci", "sub"), 0o755))
	entry := filepath.Join(dir, "Taskfile.yml")
	require.NoError(t, os.WriteFile(entry, []byte(`version: '3'
includes:
  a: ci/a.yml
tasks: {hi: {cmds: ['echo hi']}}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ci", "a.yml"), []byte(`version: '3'
includes:
  b: sub/b.yml
tasks: {from-a: {cmds: ['echo A']}}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ci", "sub", "b.yml"),
		[]byte("version: '3'\ntasks: {from-b: {cmds: ['echo B']}}\n"), 0o644))

	_, err := task.Migrate(entry, io.Discard)
	require.NoError(t, err)

	aBody, err := os.ReadFile(filepath.Join(dir, "ci", "a.Ritefile.yml"))
	require.NoError(t, err)
	if !strings.Contains(string(aBody), "b: sub/b.Ritefile.yml") {
		t.Errorf("a's includes not rewritten to point at b.Ritefile.yml\nGOT:\n%s", aBody)
	}
	if _, err := os.Stat(filepath.Join(dir, "ci", "sub", "b.Ritefile.yml")); err != nil {
		t.Errorf("expected migrated grandchild at ci/sub/b.Ritefile.yml: %v", err)
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

// TestMigrateDotenvEntryCollisions locks in the #45 fix: the DOTENV-ENTRY
// warning has to flag task-level `dotenv:` keys that collide with any
// entrypoint-level env source, not just the explicit `env:` map. Upstream
// go-task let a task-level dotenv shadow an entrypoint-level dotenv; under
// rite's first-in-wins precedence the entrypoint wins and the task-level
// key is silently dropped. Each case asserts the warning fires with the
// right source label so users can fix the authoritative declaration site.
func TestMigrateDotenvEntryCollisions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		entryEnv    string // value for the entrypoint `env:` block (empty = omitted)
		entryDotenv string // contents of an entrypoint-level .env file (empty = no dotenv ref)
		taskDotenv  string // contents of the task-level .env file
		wantSource  string // expected source label in the warning
	}{
		{
			name:       "entrypoint_env_only",
			entryEnv:   "CONFIG: from-env\n",
			taskDotenv: "CONFIG=from-task-dotenv\n",
			wantSource: "env",
		},
		{
			name:        "entrypoint_dotenv_only",
			entryDotenv: "CONFIG=from-entry-dotenv\n",
			taskDotenv:  "CONFIG=from-task-dotenv\n",
			wantSource:  "dotenv",
		},
		{
			name:        "entrypoint_env_and_dotenv",
			entryEnv:    "CONFIG: from-env\n",
			entryDotenv: "CONFIG=from-entry-dotenv\n",
			taskDotenv:  "CONFIG=from-task-dotenv\n",
			wantSource:  "env+dotenv",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.task"),
				[]byte(c.taskDotenv), 0o644))

			var envBlock, entryDotenvRef string
			if c.entryEnv != "" {
				envBlock = "env:\n  " + c.entryEnv
			}
			if c.entryDotenv != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.entry"),
					[]byte(c.entryDotenv), 0o644))
				entryDotenvRef = "dotenv: ['.env.entry']\n"
			}

			src := filepath.Join(dir, "Taskfile.yml")
			body := "version: '3'\n" + envBlock + entryDotenvRef + `tasks:
  t:
    dotenv: ['.env.task']
    cmds:
      - echo hi
`
			require.NoError(t, os.WriteFile(src, []byte(body), 0o644))

			var warn bytes.Buffer
			_, err := task.Migrate(src, &warn)
			require.NoError(t, err)

			got := warn.String()
			wantFrag := "DOTENV-ENTRY"
			if !strings.Contains(got, wantFrag) {
				t.Fatalf("warning missing %q\nGOT:\n%s", wantFrag, got)
			}
			wantSourceFrag := "entrypoint " + c.wantSource + " —"
			if !strings.Contains(got, wantSourceFrag) {
				t.Errorf("warning missing source label %q\nGOT:\n%s", wantSourceFrag, got)
			}
		})
	}
}

// TestMigrateDotenvEntryNoFalsePositive: a task-level dotenv key that
// doesn't collide with any entrypoint-level env source must not trigger
// DOTENV-ENTRY, even when the entrypoint does declare env/dotenv for
// unrelated keys.
func TestMigrateDotenvEntryNoFalsePositive(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.entry"),
		[]byte("ALPHA=a\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env.task"),
		[]byte("BETA=b\n"), 0o644))

	src := filepath.Join(dir, "Taskfile.yml")
	body := `version: '3'
env:
  GAMMA: g
dotenv: ['.env.entry']
tasks:
  t:
    dotenv: ['.env.task']
    cmds:
      - echo hi
`
	require.NoError(t, os.WriteFile(src, []byte(body), 0o644))

	var warn bytes.Buffer
	_, err := task.Migrate(src, &warn)
	require.NoError(t, err)

	if strings.Contains(warn.String(), "DOTENV-ENTRY") {
		t.Errorf("unexpected DOTENV-ENTRY warning\nGOT:\n%s", warn.String())
	}
}
