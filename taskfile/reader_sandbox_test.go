package taskfile_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clintmod/rite/errors"
	"github.com/clintmod/rite/taskfile"
)

// writeFile writes content to path, creating parent dirs as needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readFrom constructs a root node at root/Ritefile.yml and runs Reader.Read.
// Chdir mutates process state so callers must not use t.Parallel().
func readFrom(t *testing.T, root string) error {
	t.Helper()
	entrypoint := filepath.Join(root, "Ritefile.yml")
	t.Chdir(root)

	node, err := taskfile.NewRootNode(entrypoint, root)
	if err != nil {
		return err
	}
	_, err = taskfile.NewReader().Read(context.Background(), node)
	return err
}

// absolutePathForTest returns a platform-correct absolute path to reference
// in include declarations. `/etc/hosts` is absolute on Unix but relative on
// Windows (Windows absolute paths need a drive letter). filepath.Abs gives
// us `/etc/hosts` on Unix and `C:\etc\hosts` on Windows.
func absolutePathForTest(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.FromSlash("/etc/hosts"))
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	return abs
}

//nolint:paralleltest // chdir; see readFrom
func TestIncludeRejectsAbsolutePath(t *testing.T) {
	root := t.TempDir()
	abs := absolutePathForTest(t)
	// Single-quoted YAML scalar — backslashes (Windows) pass through untouched.
	writeFile(t, filepath.Join(root, "Ritefile.yml"), fmt.Sprintf(`version: '3'
includes:
  evil:
    taskfile: '%s'
tasks:
  default:
    cmds:
      - echo hi
`, abs))

	err := readFrom(t, root)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var escape errors.IncludeEscapesTreeError
	if !errors.As(err, &escape) {
		t.Fatalf("expected IncludeEscapesTreeError, got %T: %v", err, err)
	}
}

//nolint:paralleltest // chdir; see readFrom
func TestIncludeRejectsAbsolutePathEvenOptional(t *testing.T) {
	root := t.TempDir()
	abs := absolutePathForTest(t)
	writeFile(t, filepath.Join(root, "Ritefile.yml"), fmt.Sprintf(`version: '3'
includes:
  evil:
    taskfile: '%s'
    optional: true
tasks:
  default:
    cmds:
      - echo hi
`, abs))

	err := readFrom(t, root)
	if err == nil {
		t.Fatal("expected error, got nil — optional should not suppress a traversal attempt")
	}
	var escape errors.IncludeEscapesTreeError
	if !errors.As(err, &escape) {
		t.Fatalf("expected IncludeEscapesTreeError, got %T: %v", err, err)
	}
}

//nolint:paralleltest // chdir; see readFrom
func TestIncludeRejectsRelativeEscape(t *testing.T) {
	// Create an "outside" file that a `../../evil.yml` could point at.
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	writeFile(t, filepath.Join(parent, "evil.yml"), `version: '3'
tasks:
  pwn:
    cmds: [echo pwned]
`)
	writeFile(t, filepath.Join(root, "Ritefile.yml"), `version: '3'
includes:
  evil:
    taskfile: ../evil.yml
tasks:
  default:
    cmds:
      - echo hi
`)

	err := readFrom(t, root)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var escape errors.IncludeEscapesTreeError
	if !errors.As(err, &escape) {
		t.Fatalf("expected IncludeEscapesTreeError, got %T: %v", err, err)
	}
}

//nolint:paralleltest // chdir; see readFrom
func TestIncludeRejectsSymlinkEscape(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	outside := filepath.Join(parent, "outside.yml")
	writeFile(t, outside, `version: '3'
tasks:
  pwn:
    cmds: [echo pwned]
`)
	writeFile(t, filepath.Join(root, "Ritefile.yml"), `version: '3'
includes:
  evil:
    taskfile: inside.yml
tasks:
  default:
    cmds:
      - echo hi
`)
	// inside.yml is a symlink pointing outside the tree
	if err := os.Symlink(outside, filepath.Join(root, "inside.yml")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	err := readFrom(t, root)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var escape errors.IncludeEscapesTreeError
	if !errors.As(err, &escape) {
		t.Fatalf("expected IncludeEscapesTreeError, got %T: %v", err, err)
	}
}

//nolint:paralleltest // chdir; see readFrom
func TestIncludeAllowsNestedWithinTree(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sub", "nested.yml"), `version: '3'
tasks:
  hello:
    cmds: [echo hi]
`)
	writeFile(t, filepath.Join(root, "Ritefile.yml"), `version: '3'
includes:
  sub:
    taskfile: sub/nested.yml
tasks:
  default:
    cmds:
      - echo top
`)

	if err := readFrom(t, root); err != nil {
		t.Fatalf("legitimate nested include rejected: %v", err)
	}
}

//nolint:paralleltest // chdir; see readFrom
func TestIncludeAllowsInTreeSymlink(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real.yml")
	writeFile(t, real, `version: '3'
tasks:
  hello:
    cmds: [echo hi]
`)
	writeFile(t, filepath.Join(root, "Ritefile.yml"), `version: '3'
includes:
  sym:
    taskfile: link.yml
tasks:
  default:
    cmds:
      - echo top
`)
	if err := os.Symlink(real, filepath.Join(root, "link.yml")); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	if err := readFrom(t, root); err != nil {
		t.Fatalf("in-tree symlink include rejected: %v", err)
	}
}

//nolint:paralleltest // chdir; see readFrom
func TestDecodeErrorRedactsNonRitefileSnippet(t *testing.T) {
	// A file that is NOT named Ritefile-ish, with type-mismatched YAML
	// that triggers the RitefileDecodeError (snippet-building) branch.
	// Simulate by pointing the root entrypoint at a weird name.
	root := t.TempDir()
	weird := filepath.Join(root, "passwd")
	secret := "root:x:0:0:root:/root:/bin/bash\n" +
		"tasks: \"this would render as a snippet if not redacted\"\n"
	writeFile(t, weird, "version: '3'\n"+secret)

	node, err := taskfile.NewRootNode(weird, root)
	if err != nil {
		t.Fatal(err)
	}
	_, err = taskfile.NewReader().Read(context.Background(), node)
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}
	msg := err.Error()
	if strings.Contains(msg, "root:x:0:0") {
		t.Fatalf("error leaked file contents: %q", msg)
	}
}

//nolint:paralleltest // chdir; see readFrom
func TestDecodeErrorKeepsSnippetForRitefile(t *testing.T) {
	// A file named Ritefile.yml with type-mismatched YAML should still
	// include a snippet in the error.
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "Ritefile.yml"), `version: '3'
tasks: "not-a-map"
`)

	err := readFrom(t, root)
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}
	if !strings.Contains(err.Error(), "not-a-map") {
		t.Fatalf("expected snippet in error for Ritefile, got: %q", err.Error())
	}
}
