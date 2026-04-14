package ast

import (
	"fmt"
	"os"
	"sync"

	"github.com/dominikbraun/graph"
	"github.com/dominikbraun/graph/draw"
	"golang.org/x/sync/errgroup"
)

type RitefileGraph struct {
	sync.Mutex
	graph.Graph[string, *RitefileVertex]
}

// A RitefileVertex is a vertex on the Ritefile DAG.
type RitefileVertex struct {
	URI      string
	Ritefile *Ritefile
}

func taskfileHash(vertex *RitefileVertex) string {
	return vertex.URI
}

func NewRitefileGraph() *RitefileGraph {
	return &RitefileGraph{
		sync.Mutex{},
		graph.New(taskfileHash,
			graph.Directed(),
			graph.PreventCycles(),
			graph.Rooted(),
		),
	}
}

func (tfg *RitefileGraph) Visualize(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return draw.DOT(tfg.Graph, f)
}

func (tfg *RitefileGraph) Merge() (*Ritefile, error) {
	hashes, err := graph.TopologicalSort(tfg.Graph)
	if err != nil {
		return nil, err
	}

	predecessorMap, err := tfg.PredecessorMap()
	if err != nil {
		return nil, err
	}

	// Loop over each vertex in reverse topological order except for the root vertex.
	// This gives us a loop over every included Ritefile in an order which is safe to merge.
	for i := len(hashes) - 1; i > 0; i-- {
		hash := hashes[i]

		// Get the included vertex
		includedVertex, err := tfg.Vertex(hash)
		if err != nil {
			return nil, err
		}

		// Fan out across every parent that includes this vertex, then join once
		// at the end of the level. `Vars.Merge` (issue #48) and `Tasks` Get/Set
		// are lock-protected, and distinct predecessor edges point at distinct
		// parent vertices, so the per-edge goroutines write to disjoint `t1`s.
		var g errgroup.Group

		for _, edge := range predecessorMap[hash] {
			g.Go(func() error {
				vertex, err := tfg.Vertex(edge.Source)
				if err != nil {
					return err
				}

				includes, ok := edge.Properties.Data.([]*Include)
				if !ok {
					return fmt.Errorf("rite: Failed to get merge options")
				}

				for _, include := range includes {
					if err := vertex.Ritefile.Merge(
						includedVertex.Ritefile,
						include,
					); err != nil {
						return err
					}
				}

				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return nil, err
		}
	}

	// Get the root vertex
	rootVertex, err := tfg.Vertex(hashes[0])
	if err != nil {
		return nil, err
	}

	return rootVertex.Ritefile, nil
}
