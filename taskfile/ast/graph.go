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

		// Create an error group to wait for all the included Taskfiles to be merged with all its parents
		var g errgroup.Group

		// Loop over edge that leads to a vertex that includes the current vertex
		for _, edge := range predecessorMap[hash] {

			// Start a goroutine to process each included Ritefile
			g.Go(func() error {
				// Get the base vertex
				vertex, err := tfg.Vertex(edge.Source)
				if err != nil {
					return err
				}

				// Get the merge options
				includes, ok := edge.Properties.Data.([]*Include)
				if !ok {
					return fmt.Errorf("rite: Failed to get merge options")
				}

				// Merge the included Taskfiles into the parent Ritefile
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
			if err := g.Wait(); err != nil {
				return nil, err
			}
		}

		// Wait for all the go routines to finish
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
