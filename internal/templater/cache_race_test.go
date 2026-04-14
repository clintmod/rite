package templater_test

import (
	"sync"
	"testing"

	"github.com/clintmod/rite/internal/templater"
	"github.com/clintmod/rite/taskfile/ast"
)

// TestCacheConcurrentReplace is the regression test for #52: a freshly
// constructed Cache whose cacheMap hasn't been materialized yet used to
// race when two goroutines called Replace on it simultaneously, because
// the lazy init + maps.Clone sequence in replaceImpl had no lock.
// Under `go test -race` this would fail before the fix in the same file.
func TestCacheConcurrentReplace(t *testing.T) {
	t.Parallel()
	vars := ast.NewVars()
	vars.Set("GREETING", ast.Var{Value: "hello"})
	vars.Set("NAME", ast.Var{Value: "world"})

	cache := &templater.Cache{Vars: vars}

	const goroutines = 16
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			got := templater.Replace("{{.GREETING}} {{.NAME}}", cache)
			if got != "hello world" {
				t.Errorf("Replace = %q, want %q", got, "hello world")
			}
		}()
	}
	wg.Wait()
}

// TestCacheConcurrentSeedAndReplace exercises the Seed + Replace interleave:
// one goroutine is installing fallback entries while others are resolving
// them. Both sides now acquire the Cache mutex so the cacheMap can't be
// observed mid-mutation.
func TestCacheConcurrentSeedAndReplace(t *testing.T) {
	t.Parallel()
	vars := ast.NewVars()
	vars.Set("A", ast.Var{Value: "1"})

	cache := &templater.Cache{Vars: vars}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			cache.Seed(map[string]any{"B": "2"})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = templater.Replace("{{.A}}", cache)
		}
	}()
	wg.Wait()
}
