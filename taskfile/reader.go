package taskfile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dominikbraun/graph"
	"go.yaml.in/yaml/v3"
	"golang.org/x/sync/errgroup"

	"github.com/clintmod/rite/errors"
	"github.com/clintmod/rite/internal/env"
	"github.com/clintmod/rite/internal/filepathext"
	"github.com/clintmod/rite/internal/templater"
	"github.com/clintmod/rite/taskfile/ast"
)

type (
	// DebugFunc is a function that can be called to log debug messages.
	DebugFunc func(string)
	// A ReaderOption is any type that can apply a configuration to a [Reader].
	ReaderOption interface {
		ApplyToReader(*Reader)
	}
	// A Reader recursively reads Ritefiles from a given [Node] and builds a
	// [ast.RitefileGraph] from them.
	Reader struct {
		graph     *ast.RitefileGraph
		debugFunc DebugFunc
		// sandboxRoot is the absolute path used to scope `includes:` paths.
		// It is the wider of (process cwd, dir of root Ritefile) so that:
		//   - a sibling include `../included.yml` inside the user's project
		//     resolves cleanly (still inside cwd),
		//   - includes pointing at `/etc/passwd` or escaping cwd via `../`
		//     get rejected as IncludeEscapesTreeError.
		// Set lazily on the first include() call (when node.Parent() is nil).
		sandboxRoot     string
		sandboxRootOnce sync.Once
	}
)

// NewReader constructs a new Ritefile [Reader] using the given options.
func NewReader(opts ...ReaderOption) *Reader {
	r := &Reader{
		graph: ast.NewRitefileGraph(),
	}
	r.Options(opts...)
	return r
}

// Options loops through the given [ReaderOption] functions and applies them to
// the [Reader].
func (r *Reader) Options(opts ...ReaderOption) {
	for _, opt := range opts {
		opt.ApplyToReader(r)
	}
}

// WithDebugFunc sets the debug function to be used by the [Reader]. If set,
// this function will be called with debug messages. By default, no debug
// function is set and the logs are not written.
func WithDebugFunc(debugFunc DebugFunc) ReaderOption {
	return &debugFuncOption{debugFunc: debugFunc}
}

type debugFuncOption struct {
	debugFunc DebugFunc
}

func (o *debugFuncOption) ApplyToReader(r *Reader) {
	r.debugFunc = o.debugFunc
}

// Read will read the Ritefile defined by the given [Node] and recurse through
// any [ast.Includes] it finds, reading each included Ritefile and building an
// [ast.RitefileGraph] as it goes.
func (r *Reader) Read(ctx context.Context, node Node) (*ast.RitefileGraph, error) {
	if err := r.include(ctx, node); err != nil {
		return nil, err
	}
	return r.graph, nil
}

func (r *Reader) include(ctx context.Context, node Node) error {
	// The first call to include() is always the root Ritefile. Pick the
	// sandbox root now so every subsequent `includes:` entry can be checked
	// for containment.
	if node.Parent() == nil {
		r.sandboxRootOnce.Do(func() {
			r.sandboxRoot = computeSandboxRoot(node.Dir())
		})
	}

	vertex := &ast.RitefileVertex{
		URI:      node.Location(),
		Ritefile: nil,
	}

	if err := r.graph.AddVertex(vertex); err == graph.ErrVertexAlreadyExists {
		return nil
	} else if err != nil {
		return err
	}

	var err error
	vertex.Ritefile, err = r.readNode(node)
	if err != nil {
		return err
	}

	var g errgroup.Group

	for _, include := range vertex.Ritefile.Includes.All() {
		vars := env.GetEnviron()
		vars.Merge(vertex.Ritefile.Vars, nil)
		g.Go(func() error {
			cache := &templater.Cache{Vars: vars}
			include = &ast.Include{
				Namespace:      include.Namespace,
				Ritefile:       templater.Replace(include.Ritefile, cache),
				Dir:            templater.Replace(include.Dir, cache),
				Optional:       include.Optional,
				Internal:       include.Internal,
				Flatten:        include.Flatten,
				Aliases:        include.Aliases,
				AdvancedImport: include.AdvancedImport,
				Excludes:       include.Excludes,
				Vars:           include.Vars,
				Checksum:       include.Checksum,
			}
			if err := cache.Err(); err != nil {
				return err
			}

			if hasURLScheme(include.Ritefile) {
				if include.Optional {
					return nil
				}
				return errors.New("rite: remote Ritefiles are not supported — check Ritefiles in to your repo to keep task execution idempotent")
			}

			// Sandbox layer 1a: reject absolute paths in `includes:`.
			// Optional doesn't suppress this — a traversal attempt is an
			// attack signal, not a missing-file scenario.
			if filepathext.IsAbs(include.Ritefile) {
				return errors.IncludeEscapesTreeError{
					IncludePath: include.Ritefile,
					Reason:      "absolute paths are not permitted in includes",
				}
			}

			entrypoint, err := node.ResolveEntrypoint(include.Ritefile)
			if err != nil {
				return err
			}

			// Sandbox layer 1b: lexical containment. The resolved entrypoint
			// must sit inside the sandbox root. Catches `../../../etc/hosts`
			// and similar.
			if err := r.validateWithinSandbox(include.Ritefile, entrypoint); err != nil {
				return err
			}

			include.Dir, err = node.ResolveDir(include.Dir)
			if err != nil {
				return err
			}

			// Sandbox layer 1c: symlink containment. If the file exists and
			// resolves (via symlink) to a target outside the sandbox, reject.
			// If the file doesn't exist, fall through — NewNode below will
			// produce a NotExist error that Optional can suppress with the
			// correct error type.
			if err := r.validateSymlinksWithinSandbox(include.Ritefile, entrypoint); err != nil {
				return err
			}

			includeNode, err := NewNode(entrypoint, include.Dir,
				WithParent(node),
				WithChecksum(include.Checksum),
			)
			if err != nil {
				if include.Optional {
					return nil
				}
				return err
			}

			if err := r.include(ctx, includeNode); err != nil {
				return err
			}

			r.graph.Lock()
			defer r.graph.Unlock()
			edge, err := r.graph.Edge(node.Location(), includeNode.Location())
			if err == graph.ErrEdgeNotFound {
				err = r.graph.AddEdge(
					node.Location(),
					includeNode.Location(),
					graph.EdgeData([]*ast.Include{include}),
					graph.EdgeWeight(1),
				)
			} else {
				edgeData := append(edge.Properties.Data.([]*ast.Include), include)
				err = r.graph.UpdateEdge(
					node.Location(),
					includeNode.Location(),
					graph.EdgeData(edgeData),
					graph.EdgeWeight(len(edgeData)),
				)
			}
			if errors.Is(err, graph.ErrEdgeCreatesCycle) {
				return errors.RitefileCycleError{
					Source:      node.Location(),
					Destination: includeNode.Location(),
				}
			}
			return err
		})
	}

	return g.Wait()
}

func (r *Reader) readNode(node Node) (*ast.Ritefile, error) {
	b, err := node.Read()
	if err != nil {
		return nil, err
	}

	sum := sha256Hex(b)
	if !node.Verify(sum) {
		return nil, &errors.RitefileDoesNotMatchChecksum{
			URI:              node.Location(),
			ExpectedChecksum: node.Checksum(),
			ActualChecksum:   sum,
		}
	}

	var tf ast.Ritefile
	if err := yaml.Unmarshal(b, &tf); err != nil {
		taskfileDecodeErr := &errors.RitefileDecodeError{}
		if errors.As(err, &taskfileDecodeErr) {
			// Sandbox layer 2: suppress the file-content snippet when the
			// target doesn't look like a Ritefile/Taskfile. Prevents
			// `rite -t /etc/passwd` (or any mis-pointed flag) from echoing
			// the target's contents into a decode error.
			var snippetStr string
			if looksLikeRitefileName(node.Location()) {
				snippetStr = NewSnippet(b,
					WithLine(taskfileDecodeErr.Line),
					WithColumn(taskfileDecodeErr.Column),
					WithPadding(2),
				).String()
			}
			return nil, taskfileDecodeErr.WithFileInfo(node.Location(), snippetStr)
		}
		return nil, &errors.RitefileInvalidError{URI: filepathext.TryAbsToRel(node.Location()), Err: err}
	}

	if tf.Version == nil {
		return nil, &errors.RitefileVersionCheckError{URI: node.Location()}
	}

	tf.Location = node.Location()
	for task := range tf.Tasks.Values(nil) {
		if task == nil {
			task = &ast.Task{}
		}
		if task.Location.Ritefile == "" {
			task.Location.Ritefile = tf.Location
		}
	}

	return &tf, nil
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// computeSandboxRoot picks the widest reasonable anchor for the includes
// sandbox: the union of the process working directory and the dir of the
// root Ritefile. When one is an ancestor of the other we pick the
// ancestor (so legitimate `../sibling.yml` includes in projects run from
// anywhere inside the repo keep working). When the two are unrelated
// (e.g. `rite -t /elsewhere/Ritefile.yml` run from `/home/me/project`)
// we fall back to the root Ritefile's dir so `-t` still works cleanly.
// All errors fall back to rootDir — containment checks still run, they're
// just lexically anchored at the root node's dir.
func computeSandboxRoot(rootDir string) string {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return rootDir
	}
	cwd, err := os.Getwd()
	if err != nil {
		return absRoot
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return absRoot
	}
	if isAncestorOrEqual(absCwd, absRoot) {
		return absCwd
	}
	if isAncestorOrEqual(absRoot, absCwd) {
		return absRoot
	}
	return absRoot
}

// isAncestorOrEqual reports whether child is parent itself or lives
// beneath it. Both inputs must already be absolute paths.
func isAncestorOrEqual(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel)
}

// validateWithinSandbox rejects resolved include entrypoints that sit
// outside the sandbox root. Uses filepath.Rel: a path whose Rel starts
// with ".." (or equals "..") is outside. If sandboxRoot isn't set we
// fail open with nil so callers that bypass Reader.Read aren't broken;
// the CLI entry point always calls Read with a root node first.
func (r *Reader) validateWithinSandbox(original, resolved string) error {
	if r.sandboxRoot == "" {
		return nil
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return errors.IncludeEscapesTreeError{
			IncludePath: original,
			Reason:      "could not resolve to an absolute path",
		}
	}
	if !isAncestorOrEqual(r.sandboxRoot, abs) {
		return errors.IncludeEscapesTreeError{
			IncludePath: original,
			Reason:      "path resolves outside the project tree",
		}
	}
	return nil
}

// validateSymlinksWithinSandbox resolves any symlinks on the include path
// and re-checks tree containment. Closes the "symlink inside the tree
// points outside" escape. If the file doesn't exist EvalSymlinks errors —
// we ignore that here and let the normal load path surface NotExist
// (which Optional can suppress) with the correct error type.
func (r *Reader) validateSymlinksWithinSandbox(original, resolved string) error {
	if r.sandboxRoot == "" {
		return nil
	}
	real, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		return nil
	}
	absReal, err := filepath.Abs(real)
	if err != nil {
		return errors.IncludeEscapesTreeError{
			IncludePath: original,
			Reason:      "could not resolve symlinks to an absolute path",
		}
	}
	// EvalSymlinks the sandbox root too so both sides compare in the same
	// form (on macOS /tmp -> /private/tmp, etc.).
	absRoot := r.sandboxRoot
	if evaled, err := filepath.EvalSymlinks(r.sandboxRoot); err == nil {
		absRoot = evaled
	}
	if !isAncestorOrEqual(absRoot, absReal) {
		return errors.IncludeEscapesTreeError{
			IncludePath: original,
			Reason:      "symlink resolves outside the project tree",
		}
	}
	return nil
}

// looksLikeRitefileName decides whether a path is "Ritefile-ish" enough
// to justify embedding a content snippet in a parse-error message. Accepts
// basenames starting with "ritefile"/"taskfile" (case-insensitive) or with
// a .yml/.yaml extension. Anything else — `/etc/passwd`, binaries mis-aimed
// via `-t` — gets the snippet suppressed to avoid leaking file contents.
func looksLikeRitefileName(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	if strings.HasPrefix(base, "ritefile") || strings.HasPrefix(base, "taskfile") {
		return true
	}
	ext := strings.ToLower(filepath.Ext(base))
	return ext == ".yml" || ext == ".yaml"
}
