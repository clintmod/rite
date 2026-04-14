package ast

import (
	"strconv"
	"sync"
	"testing"
)

// TestVarsMergeConcurrent race-fences issue #48: Vars.Merge used to write
// into the destination orderedmap without acquiring the destination's mutex.
// Today graph.Merge serializes the include walk so the bug is quiet, but
// any code that Merges into a shared *Vars from two goroutines trips the
// race detector. Concretely: a reader on the destination + a Merge writer
// racing on the same orderedmap is exactly the pattern `go test -race`
// flags.
//
// Run with `go test -race ./taskfile/ast/ -run TestVarsMerge` to reproduce
// against the pre-fix code.
func TestVarsMergeConcurrent(t *testing.T) {
	t.Parallel()

	dst := NewVars()
	dst.Set("seed", Var{Value: "initial"})

	const writers = 8
	const perWriter = 200

	var wg sync.WaitGroup
	wg.Add(writers * 2)

	// Writers: N goroutines each merging a fresh `other` into `dst`.
	for i := 0; i < writers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < perWriter; j++ {
				other := NewVars()
				other.Set("w"+strconv.Itoa(id)+"_"+strconv.Itoa(j), Var{Value: j})
				dst.Merge(other, nil)
			}
		}(i)
	}

	// Readers: N goroutines hammering Get on dst. Reads are protected by
	// the RWMutex, so with a properly locking Merge they observe a
	// consistent ordered map; without the fix, the race detector flags
	// concurrent writes into om alongside these reads.
	for i := 0; i < writers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perWriter; j++ {
				_, _ = dst.Get("seed")
			}
		}()
	}

	wg.Wait()

	// Sanity check: after all writers drain, every expected key should be
	// present. (Loss here would mean some Merge calls silently dropped
	// writes — not the primary defect we're fencing, but cheap to assert.)
	expected := writers * perWriter
	got := 0
	for k := range dst.All() {
		if k == "seed" {
			continue
		}
		got++
	}
	if got != expected {
		t.Fatalf("expected %d merged keys, got %d", expected, got)
	}
}

// TestVarsMergeSelf checks that Merging a Vars into itself doesn't deadlock.
// The fix releases `other`'s read lock before acquiring `vars`' write lock,
// so same-pointer merges are safe — regression fence in case someone
// tightens the locking back up to "hold both" and reintroduces the deadlock.
func TestVarsMergeSelf(t *testing.T) {
	t.Parallel()

	v := NewVars()
	v.Set("a", Var{Value: 1})
	v.Set("b", Var{Value: 2})

	done := make(chan struct{})
	go func() {
		v.Merge(v, nil)
		close(done)
	}()

	<-done

	if _, ok := v.Get("a"); !ok {
		t.Fatal("expected key 'a' to survive self-merge")
	}
	if _, ok := v.Get("b"); !ok {
		t.Fatal("expected key 'b' to survive self-merge")
	}
}
