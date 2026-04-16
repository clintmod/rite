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

	// Schema directive is rewritten to rite's hosted schema (#72).
	if strings.Contains(got, "taskfile.dev") {
		t.Errorf("schema URL still references taskfile.dev\nGOT:\n%s", got)
	}
	if !strings.Contains(got, "https://clintmod.github.io/rite/schema/v3.json") {
		t.Errorf("schema URL not rewritten to rite's hosted schema\nGOT:\n%s", got)
	}

	// Check warnings.
	warnS := warn.String()
	wantWarns := []string{
		"OVERRIDE-VAR", // GLOBAL shadowed
		"OVERRIDE-ENV", // NODE_ENV shadowed
		"SECRET-VAR",   // GITHUB_TOKEN name pattern
	}
	for _, w := range wantWarns {
		if !strings.Contains(warnS, w) {
			t.Errorf("warnings missing %q\nGOT:\n%s", w, warnS)
		}
	}
	// SCHEMA-URL is no longer a warning class — migrate fixes it in place.
	if strings.Contains(warnS, "SCHEMA-URL") {
		t.Errorf("unexpected SCHEMA-URL warning (migrate should rewrite, not warn)\nGOT:\n%s", warnS)
	}
}

// TestMigrateRewritesSchemaPointer exercises #72: the yaml-language-server
// schema directive should be rewritten from taskfile.dev → clintmod.github.io
// across whitespace, quoting, and path variants. Non-taskfile.dev URLs and
// files without the directive are left alone.
func TestMigrateRewritesSchemaPointer(t *testing.T) {
	t.Parallel()

	const riteURL = "https://clintmod.github.io/rite/schema/v3.json"

	cases := []struct {
		name     string
		in       string
		wantHave string // substring that must appear
		wantGone string // substring that must NOT appear (empty = no check)
	}{
		{
			name:     "canonical go-task directive",
			in:       "# yaml-language-server: $schema=https://taskfile.dev/schema.json\n",
			wantHave: "$schema=" + riteURL,
			wantGone: "taskfile.dev",
		},
		{
			name:     "double-quoted URL",
			in:       `# yaml-language-server: $schema="https://taskfile.dev/schema.json"` + "\n",
			wantHave: `$schema="` + riteURL + `"`,
			wantGone: "taskfile.dev",
		},
		{
			name:     "single-quoted URL",
			in:       `# yaml-language-server: $schema='https://taskfile.dev/schema.json'` + "\n",
			wantHave: `$schema='` + riteURL + `'`,
			wantGone: "taskfile.dev",
		},
		{
			name:     "extra whitespace around colon and equals",
			in:       "#   yaml-language-server:   $schema =  https://taskfile.dev/schema.json\n",
			wantHave: riteURL,
			wantGone: "taskfile.dev",
		},
		{
			name:     "versioned path at taskfile.dev is still rewritten",
			in:       "# yaml-language-server: $schema=https://taskfile.dev/v3/schema.json\n",
			wantHave: riteURL,
			wantGone: "taskfile.dev",
		},
		{
			name:     "indented directive (inside an included file)",
			in:       "  # yaml-language-server: $schema=https://taskfile.dev/schema.json\n",
			wantHave: "  # yaml-language-server: $schema=" + riteURL,
			wantGone: "taskfile.dev",
		},
		{
			name:     "non-taskfile.dev URL is untouched",
			in:       "# yaml-language-server: $schema=https://json.schemastore.org/taskfile.json\n",
			wantHave: "https://json.schemastore.org/taskfile.json",
			wantGone: "clintmod.github.io",
		},
		{
			name:     "no directive present — nothing injected",
			in:       "version: '3'\ntasks:\n  default:\n    cmds: [echo hi]\n",
			wantHave: "version: '3'",
			wantGone: "yaml-language-server",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			src := filepath.Join(dir, "Taskfile.yml")
			body := tc.in + "version: '3'\ntasks:\n  default:\n    cmds: [echo ok]\n"
			if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			dst, err := task.Migrate(src, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(dst)
			if err != nil {
				t.Fatal(err)
			}
			gs := string(got)
			if !strings.Contains(gs, tc.wantHave) {
				t.Errorf("missing %q\nGOT:\n%s", tc.wantHave, gs)
			}
			if tc.wantGone != "" && strings.Contains(gs, tc.wantGone) {
				t.Errorf("should not contain %q\nGOT:\n%s", tc.wantGone, gs)
			}
		})
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

	// The special-var rename (.TASK → .RITE_NAME, .TASK_DIR → .RITE_TASK_DIR)
	// feeds into the modernization pass, so the final on-disk form is the
	// `${VAR}` shell-preprocessor surface. Both passes together are what
	// users see when they run `rite migrate` — this test locks that in.
	wantFrags := []string{
		"echo ${RITE_NAME}",
		"running ${RITE_NAME} in ${RITE_TASK_DIR}",
		"${MY_TASK_DIR}",       // user var modernizes too (bare ref)
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
		"{{.TASK",         // no stale Go-template special refs
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

	// As with TestMigrateRewritesSpecialVars, the rename pass feeds the
	// modernization pass, so the final output uses `${VAR}` form.
	wantFrags := []string{
		`${RITEFILE} lives in ${RITEFILE_DIR}`,
		`root is ${ROOT_RITEFILE}`,
		`version ${RITE_VERSION}`,
		`${MY_TASKFILE}`,           // user var modernizes too
		`# .TASKFILE in a comment`, // non-template occurrence stays untouched
	}
	for _, w := range wantFrags {
		if !strings.Contains(got, w) {
			t.Errorf("output missing %q\nGOT:\n%s", w, got)
		}
	}

	unwantFrags := []string{
		".TASKFILE}",       // old name should not survive inside a template
		".TASKFILE_DIR}",   // ditto
		".ROOT_TASKFILE}",  // ditto
		".TASK_VERSION}",   // ditto
		"{{.RITEFILE",      // modernized away
		"{{.ROOT_RITEFILE", // modernized away
		"{{.RITE_VERSION",  // modernized away
		".RITEFILE_DIR_",   // no cascading
		".ROOT_RITEFILE_",  // no cascading
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
			// Adjacent single-ref expressions get both renamed AND
			// modernized to the shell-preprocessor form — each
			// `{{ .VAR }}` is a bare ref so the modernization pass
			// rewrites it to `${VAR}`.
			name: "separate_expressions_still_work",
			cmd:  `echo {{.TASK}}{{.TASK_DIR}}`,
			want: []string{"${RITE_NAME}${RITE_TASK_DIR}"},
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

// TestMigrateModernizesTemplates locks in the #74 rewrite table: safe
// Go-template variable shapes become rite-native `${VAR}` /
// `${VAR:-fallback}` form during migrate. Both halves resolve through
// the same templater pipeline (Go-template first, then ExpandShell, then
// mvdan/sh for cmd exec), so the semantics round-trip for exported vars
// — which is the common case and the only one `rite migrate` claims to
// rewrite safely.
func TestMigrateModernizesTemplates(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, cmd, want string
	}{
		{"bare", `echo {{.FOO}}`, `echo ${FOO}`},
		{"bare_whitespace", `echo {{ .FOO }}`, `echo ${FOO}`},
		{"bare_trim_markers", `echo {{- .FOO -}}`, `echo ${FOO}`},
		{"pipe_default_double", `echo {{.FOO | default "bar"}}`, `echo ${FOO:-bar}`},
		{"pipe_default_single", `echo {{.FOO | default 'bar'}}`, `echo ${FOO:-bar}`},
		{"pipe_default_dotted", `echo {{.FOO | default .OTHER}}`, `echo ${FOO:-${OTHER}}`},
		{"prefix_default_double", `echo {{default "bar" .FOO}}`, `echo ${FOO:-bar}`},
		{"prefix_default_dotted", `echo {{default .OTHER .FOO}}`, `echo ${FOO:-${OTHER}}`},
		{"pipe_default_whitespace", `echo {{ .FOO  |  default  "bar" }}`, `echo ${FOO:-bar}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			src := filepath.Join(dir, "Taskfile.yml")
			input := "version: '3'\ntasks:\n  t:\n    cmds:\n      - " + c.cmd + "\n"
			require.NoError(t, os.WriteFile(src, []byte(input), 0o644))
			dst, err := task.Migrate(src, io.Discard)
			require.NoError(t, err)
			out, err := os.ReadFile(dst)
			require.NoError(t, err)
			if !strings.Contains(string(out), c.want) {
				t.Errorf("output missing %q\nGOT:\n%s", c.want, string(out))
			}
			if strings.Contains(string(out), "{{") {
				t.Errorf("output still contains {{…}} — not modernized\nGOT:\n%s", string(out))
			}
		})
	}
}

// TestMigrateTemplateKeptWarning locks in the TEMPLATE-KEPT warning class:
// Go-template constructs that can't be safely rewritten to `${VAR}` form
// (if/range/printf/index/unknown helpers) are left verbatim in the output
// AND reported with a file:line label so the user can find and rewrite
// them by hand.
func TestMigrateTemplateKeptWarning(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "Taskfile.yml")
	input := `version: '3'
tasks:
  t:
    cmds:
      - echo {{if .CI}}ci{{else}}local{{end}}
      - echo {{range .ITEMS}}{{.}}{{end}}
      - echo {{printf "%s/%s" .A .B}}
      - echo {{index .MATCH 0}}
      - echo {{.FOO | upper}}
`
	require.NoError(t, os.WriteFile(src, []byte(input), 0o644))
	var warn bytes.Buffer
	dst, err := task.Migrate(src, &warn)
	require.NoError(t, err)
	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	gotS := string(got)
	warnS := warn.String()

	// All of these must survive verbatim in the output.
	for _, frag := range []string{
		`{{if .CI}}`,
		`{{else}}`,
		`{{end}}`,
		`{{range .ITEMS}}`,
		`{{printf "%s/%s" .A .B}}`,
		`{{index .MATCH 0}}`,
		`{{.FOO | upper}}`,
	} {
		if !strings.Contains(gotS, frag) {
			t.Errorf("output missing kept template %q\nGOT:\n%s", frag, gotS)
		}
	}

	// One warning per kept expression, tagged with file:line.
	if !strings.Contains(warnS, "TEMPLATE-KEPT") {
		t.Fatalf("expected TEMPLATE-KEPT warnings\nGOT:\n%s", warnS)
	}
	// Spot-check: at least one warning points at each line we planted.
	for _, line := range []string{":5:", ":6:", ":7:", ":8:", ":9:"} {
		if !strings.Contains(warnS, line) {
			t.Errorf("expected TEMPLATE-KEPT warning referencing line %q\nGOT:\n%s", line, warnS)
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

// TestMigrateKeepGoTemplatesOptOut locks in the --keep-go-templates
// escape hatch: users who want the old template surface preserved opt
// out and get a migrate that only handles the path/special-var
// rewrites, leaving every `{{…}}` expression intact.
func TestMigrateKeepGoTemplatesOptOut(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "Taskfile.yml")
	input := `version: '3'
tasks:
  t:
    cmds:
      - echo {{.FOO}}
      - echo {{.BAR | default "x"}}
`
	require.NoError(t, os.WriteFile(src, []byte(input), 0o644))
	var warn bytes.Buffer
	dst, err := task.Migrate(src, &warn, task.WithKeepGoTemplates(true))
	require.NoError(t, err)
	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	gotS := string(got)

	for _, frag := range []string{`{{.FOO}}`, `{{.BAR | default "x"}}`} {
		if !strings.Contains(gotS, frag) {
			t.Errorf("output missing %q — opt-out did not preserve\nGOT:\n%s", frag, gotS)
		}
	}
	// No TEMPLATE-KEPT warning: the flag suppresses the whole pass.
	if strings.Contains(warn.String(), "TEMPLATE-KEPT") {
		t.Errorf("unexpected TEMPLATE-KEPT warning under opt-out\nGOT:\n%s", warn.String())
	}
}

// TestMigrateModernizeIdempotent: running migrate on an already-migrated
// file is a no-op. The `${VAR}` form produces no `{{…}}` matches, so
// the modernization pass can't do anything on a second pass — but we
// lock it in regardless because "idempotent migrate" is a user-visible
// contract for re-running against partial conversions.
func TestMigrateModernizeIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "Taskfile.yml")
	input := `version: '3'
tasks:
  t:
    cmds:
      - echo {{.FOO}}
      - echo ${ALREADY}
`
	require.NoError(t, os.WriteFile(src, []byte(input), 0o644))
	first, err := task.Migrate(src, io.Discard)
	require.NoError(t, err)
	firstBytes, err := os.ReadFile(first)
	require.NoError(t, err)

	// Point migrate at the migrated file; should produce identical bytes.
	second, err := task.Migrate(first, io.Discard)
	require.NoError(t, err)
	secondBytes, err := os.ReadFile(second)
	require.NoError(t, err)

	if string(firstBytes) != string(secondBytes) {
		t.Errorf("second migrate changed content — not idempotent\nFIRST:\n%s\nSECOND:\n%s", firstBytes, secondBytes)
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

// TestRewriteSelfRefCmds drives the shell-cmd rewriter directly with the
// edge cases enumerated in #128. The rewriter must touch only the *head*
// of a CallExpr — substring matches (`mytask`), filenames (`./task`), and
// non-command occurrences (`echo task is cool`) all stay verbatim.
//
// The rewriter goes through mvdan/sh's parser and printer; the printer is
// canonical, not preserving, so backticks become `$(…)` and some
// whitespace inside `$(…)` may shift. We assert on the substantive
// rewrite (`task` → `rite` at the head position) and on the no-rewrite
// invariants, not on byte-for-byte formatting.
func TestRewriteSelfRefCmds(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, in, want string
		changed        bool
	}{
		{"bare", "task foo", "rite foo", true},
		{"flag", "task --list", "rite --list", true},
		{"and_chain", "cd sub && task build", "cd sub && rite build", true},
		{"semicolon_chain", "task a; task b", "rite a; rite b", true},
		{"pipe_chain", "task list | grep build", "rite list | grep build", true},
		{"dollar_paren_subst", "$(task --version)", "$(rite --version)", true},
		// Backticks preserved (we splice at byte offsets, not reprint).
		{"backtick_subst", "`task --version`", "`rite --version`", true},
		{"nested_in_dquote", `echo "v: $(task --version)"`, `echo "v: $(rite --version)"`, true},
		{"echo_arg_unchanged", "echo task is cool", "echo task is cool", false},
		{"path_unchanged", "./task", "./task", false},
		{"absolute_path_unchanged", "/usr/local/bin/task", "/usr/local/bin/task", false},
		{"substring_unchanged", "mytask foo", "mytask foo", false},
		{"task_in_arg_unchanged", "go install github.com/go-task/task/v3/cmd/task@latest", "go install github.com/go-task/task/v3/cmd/task@latest", false},
		{"sglquoted_arg_unchanged", `echo 'task --list'`, `echo 'task --list'`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, parseOK, changed := task.RewriteShellCmdForTest(c.in)
			if !parseOK {
				t.Fatalf("parse failed for %q", c.in)
			}
			if changed != c.changed {
				t.Errorf("changed = %v, want %v (in=%q out=%q)", changed, c.changed, c.in, got)
			}
			if got != c.want {
				t.Errorf("rewrite mismatch:\n  in   = %q\n  got  = %q\n  want = %q", c.in, got, c.want)
			}
		})
	}
}

// TestMigrateRewritesSelfRefCmds end-to-ends the SELFREF-CMD pass through
// task.Migrate, asserting the YAML pre/post and the warning stream.
// Verifies:
//
//   - Bare cmds: list items are rewritten (`task --list` → `rite --list`).
//   - cmd: scalar values inside mapping shapes are rewritten.
//   - Quoted cmds round-trip (single + double).
//   - The structural `task:` YAML key (call-another-task shape) is NOT
//     touched — that's a key, not a shell cmd.
//   - One SELFREF-CMD warning per rewrite, file:line-tagged.
func TestMigrateRewritesSelfRefCmds(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "Taskfile.yml")
	body := `version: '3'
tasks:
  ci:
    cmds:
      - task lint
      - task test
      - 'task --list'
  bare-cmd:
    cmds:
      - cmd: task build
        silent: true
  call-another:
    cmds:
      - task: ci
  noop:
    cmds:
      - echo "task is the name of the binary"
      - ./task --not-the-binary
`
	require.NoError(t, os.WriteFile(src, []byte(body), 0o644))

	var warn bytes.Buffer
	dst, err := task.Migrate(src, &warn)
	require.NoError(t, err)
	out, err := os.ReadFile(dst)
	require.NoError(t, err)
	gotS := string(out)
	warnS := warn.String()

	// Positive: every shell `task` invocation rewrote to `rite`.
	for _, want := range []string{
		"- rite lint",
		"- rite test",
		"- 'rite --list'",
		"cmd: rite build",
	} {
		if !strings.Contains(gotS, want) {
			t.Errorf("output missing %q\nGOT:\n%s", want, gotS)
		}
	}

	// Negative: structural task: key, prose mentions, and ./task path stay.
	for _, keep := range []string{
		"- task: ci", // YAML key, not a shell cmd
		`echo "task is the name of the binary"`,
		"./task --not-the-binary",
	} {
		if !strings.Contains(gotS, keep) {
			t.Errorf("expected unchanged fragment %q missing\nGOT:\n%s", keep, gotS)
		}
	}

	// One warning per rewrite (4 cmds rewritten).
	got := strings.Count(warnS, "SELFREF-CMD")
	if got != 4 {
		t.Errorf("SELFREF-CMD warning count = %d, want 4\nGOT:\n%s", got, warnS)
	}
}

// TestRewriteSelfRefCmdsParseError asserts that a cmd which the shell
// parser rejects survives unchanged (no rewrite, parseOK=false). The
// migrate caller emits a warning so the user is told to review by hand.
func TestRewriteSelfRefCmdsParseError(t *testing.T) {
	t.Parallel()
	// Unterminated double-quote — shell would also reject this.
	in := `echo "task --list`
	got, parseOK, changed := task.RewriteShellCmdForTest(in)
	if parseOK {
		t.Errorf("expected parseOK=false for malformed input %q", in)
	}
	if changed {
		t.Errorf("expected changed=false for malformed input")
	}
	if got != in {
		t.Errorf("expected unchanged output, got %q", got)
	}
}
