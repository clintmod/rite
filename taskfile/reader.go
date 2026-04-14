package taskfile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

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

			entrypoint, err := node.ResolveEntrypoint(include.Ritefile)
			if err != nil {
				return err
			}

			include.Dir, err = node.ResolveDir(include.Dir)
			if err != nil {
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
			snippet := NewSnippet(b,
				WithLine(taskfileDecodeErr.Line),
				WithColumn(taskfileDecodeErr.Column),
				WithPadding(2),
			)
			return nil, taskfileDecodeErr.WithFileInfo(node.Location(), snippet.String())
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
