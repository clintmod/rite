package task_test

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"

	task "github.com/clintmod/rite/internal/task"
)

// TestMigrateKitchenSink runs `rite --migrate` against a synthetic Taskfile
// tree that exercises every migrate code path (#79) — sibling + nested + 3-
// deep includes, every warning class, every template-rewrite shape, every
// legacy special-var alias, and the URL-include + missing-optional edges.
// Regenerate goldens with:
//
//	GOLDIE_UPDATE=true GOLDIE_TEMPLATE=true go test ./internal/task/ -run TestMigrateKitchenSink
func TestMigrateKitchenSink(t *testing.T) {
	t.Parallel()
	runMigrateCorpusCase(t, "migrate_kitchen_sink")
}

// TestMigrateRealWorldCorpus feeds stripped-down real-world Taskfiles from
// third-party projects through migrate (#44) and golden-compares both the
// generated Ritefile and the stderr warning stream. Scope: prove migrate
// doesn't barf on realistic shapes — layout, plugin patterns, idioms that
// synthetic fixtures can't anticipate. Branch coverage belongs to the
// kitchen-sink above; this corpus is representative-shape smoke.
//
// Each subdir under testdata/migrate_corpus/ is a separate case with its
// own Taskfile.yml, attribution header (upstream repo + pinned SHA +
// license), and testdata/ subdir of goldens.
func TestMigrateRealWorldCorpus(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(filepath.Join("testdata", "migrate_corpus"))
	require.NoError(t, err)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			runMigrateCorpusCase(t, filepath.Join("migrate_corpus", name))
		})
	}
}

// runMigrateCorpusCase copies fixture (a directory under testdata/) into a
// tempdir, runs task.Migrate on the entrypoint, and golden-compares every
// emitted Ritefile plus the normalized warnings stream.
func runMigrateCorpusCase(t *testing.T, fixture string) {
	t.Helper()
	srcRoot := filepath.Join("testdata", fixture)
	dstRoot := t.TempDir()
	copyMigrateFixture(t, srcRoot, dstRoot)

	entrypoint := filepath.Join(dstRoot, "Taskfile.yml")
	var warn bytes.Buffer
	_, err := task.Migrate(entrypoint, &warn)
	require.NoError(t, err)

	g := goldie.New(t,
		goldie.WithFixtureDir(filepath.Join(srcRoot, "testdata")),
		goldie.WithEqualFn(NormalizedEqual),
	)

	// Golden-compare every emitted Ritefile, keyed by its path relative to
	// the entrypoint so the golden tree mirrors the source tree.
	var rels []string
	require.NoError(t, filepath.WalkDir(dstRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.Contains(filepath.Base(p), "Ritefile") {
			rel, rerr := filepath.Rel(dstRoot, p)
			if rerr != nil {
				return rerr
			}
			rels = append(rels, filepath.ToSlash(rel))
		}
		return nil
	}))
	sort.Strings(rels)

	for _, rel := range rels {
		content, err := os.ReadFile(filepath.Join(dstRoot, filepath.FromSlash(rel)))
		require.NoError(t, err, "reading %s", rel)
		g.Assert(t, rel, content)
	}

	g.Assert(t, "warnings.txt", []byte(normalizeMigrateWarnings(warn.String(), dstRoot)))

	// Issue #79 DoD: the migrated entrypoint must actually parse under
	// rite's own loader. Strip the remote-URL include (rite rejects those
	// at load; the migrate warning already tells the user to convert it
	// manually) and run Setup() on the remainder to prove the rest of the
	// tree is well-formed.
	entryPath := filepath.Join(dstRoot, "Ritefile.yml")
	body, err := os.ReadFile(entryPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(entryPath, stripRemoteInclude(body), 0o644))

	exe := task.NewExecutor(
		task.WithDir(dstRoot),
		task.WithStdout(io.Discard),
		task.WithStderr(io.Discard),
	)
	require.NoError(t, exe.Setup(), "migrated Ritefile tree must load under rite")
}

// stripRemoteInclude removes the `remote: … https://…` include block from
// a Ritefile so the rest of the tree can be parsed by rite. Remote includes
// are an explicit migrate warning ("migrate the file manually") and are
// rejected at Setup() time — we drop it locally rather than omit it from
// the fixture and lose coverage of the URL-rewrite path in the golden.
func stripRemoteInclude(body []byte) []byte {
	lines := strings.Split(string(body), "\n")
	var out []string
	skip := 0
	for _, ln := range lines {
		if skip > 0 {
			skip--
			continue
		}
		if strings.TrimSpace(ln) == "remote:" {
			skip = 1 // drop the next line (the `taskfile: https://…`)
			continue
		}
		out = append(out, ln)
	}
	return []byte(strings.Join(out, "\n"))
}

// copyMigrateFixture walks src and copies every file into dst, preserving
// structure. The sibling `testdata/` subdir (where goldens live) is skipped
// so the migrator doesn't see it.
func copyMigrateFixture(t *testing.T, src, dst string) {
	t.Helper()
	require.NoError(t, filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if rel == "testdata" {
			return fs.SkipDir
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	}))
}

// normalizeMigrateWarnings strips the randomized tempdir prefix from file
// paths embedded in the warning stream and line-sorts the result so goldens
// don't drift with walker order or temp-path entropy.
func normalizeMigrateWarnings(s, tmpDir string) string {
	// Strip the tempdir prefix from embedded paths. Path-separator
	// normalization (`\` → `/`) is deferred to NormalizedEqual at
	// compare time so we don't clobber Go-template literals like
	// `{{printf "..."}}` that %q-escaped the quote as `\"`.
	s = strings.ReplaceAll(s, tmpDir+string(filepath.Separator), "")
	s = strings.ReplaceAll(s, tmpDir, "")
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	sort.Strings(lines)
	return strings.Join(lines, "\n") + "\n"
}
