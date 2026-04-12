package templater

import (
	"bytes"
	"fmt"
	"maps"
	"strings"

	"github.com/go-task/template"

	"github.com/clintmod/rite/internal/deepcopy"
	"github.com/clintmod/rite/taskfile/ast"
)

// Cache is a help struct that allow us to call "replaceX" funcs multiple
// times, without having to check for error each time. The first error that
// happen will be assigned to r.err, and consecutive calls to funcs will just
// return the zero value.
type Cache struct {
	Vars *ast.Vars

	cacheMap map[string]any
	err      error
}

func (r *Cache) ResetCache() {
	r.cacheMap = r.Vars.ToCacheMap()
}

// Seed adds fallback entries to the templater's view that aren't part of the
// underlying Vars. Existing keys are preserved. Used to make built-in vars
// visible to template resolution while keeping them out of the canonical
// variable set until lowest-priority merge time.
func (r *Cache) Seed(m map[string]any) {
	if r.cacheMap == nil {
		if r.Vars != nil {
			r.cacheMap = r.Vars.ToCacheMap()
		} else {
			r.cacheMap = make(map[string]any)
		}
	}
	for k, v := range m {
		if _, exists := r.cacheMap[k]; !exists {
			r.cacheMap[k] = v
		}
	}
}

// Update records a key/value pair that was just added to the underlying Vars
// so the templater's view stays consistent without rebuilding from scratch.
func (r *Cache) Update(k string, v any) {
	if r.cacheMap == nil {
		if r.Vars != nil {
			r.cacheMap = r.Vars.ToCacheMap()
		} else {
			r.cacheMap = make(map[string]any)
		}
	}
	r.cacheMap[k] = v
}

func (r *Cache) Err() error {
	return r.err
}

func ResolveRef(ref string, cache *Cache) any {
	// If there is already an error, do nothing
	if cache.err != nil {
		return nil
	}

	// Initialize the cache map if it's not already initialized
	if cache.cacheMap == nil {
		cache.cacheMap = cache.Vars.ToCacheMap()
	}

	if ref == "." {
		return cache.cacheMap
	}
	t, err := template.New("resolver").Funcs(templateFuncs).Parse(fmt.Sprintf("{{%s}}", ref))
	if err != nil {
		cache.err = err
		return nil
	}
	val, err := t.Resolve(cache.cacheMap)
	if err != nil {
		cache.err = err
		return nil
	}
	return val
}

func Replace[T any](v T, cache *Cache) T {
	return ReplaceWithExtra(v, cache, nil)
}

// ReplaceNoShell is like Replace but skips the ExpandShell pass. Use this for
// strings that will be handed to a shell for interpretation (task cmd.Cmd):
// the shell resolves `$VAR` against its own env, and until Phase 4's
// vars/env unification lands, rite vars and the shell env can have different
// values for the same name. Running ExpandShell on a shell-bound string
// would pre-empt the shell with rite's var-set answer, which is premature
// — the endstate is correct but wave 3 hasn't yet collapsed the tiers.
func ReplaceNoShell[T any](v T, cache *Cache) T {
	return replaceImpl(v, cache, nil, false)
}

// ReplaceNoShellWithExtra is the for-loop iterator counterpart to
// ReplaceNoShell — same shell-bypass, but accepts per-iteration extras.
func ReplaceNoShellWithExtra[T any](v T, cache *Cache, extra map[string]any) T {
	return replaceImpl(v, cache, extra, false)
}

func ReplaceWithExtra[T any](v T, cache *Cache, extra map[string]any) T {
	return replaceImpl(v, cache, extra, true)
}

func replaceImpl[T any](v T, cache *Cache, extra map[string]any, expandShell bool) T {
	// If there is already an error, do nothing
	if cache.err != nil {
		return v
	}

	// Initialize the cache map if it's not already initialized
	if cache.cacheMap == nil {
		cache.cacheMap = cache.Vars.ToCacheMap()
	}

	// Create a copy of the cache map to avoid editing the original
	// If there is extra data, merge it with the cache map
	data := maps.Clone(cache.cacheMap)
	if extra != nil {
		maps.Copy(data, extra)
	}

	// Traverse the value and parse any template variables. Go template runs
	// first; ExpandShell runs over the result to handle SPEC §Template Syntax
	// `${VAR}` / `$VAR` / `$$` → `$` shell-native references. Both syntaxes
	// resolve against the same var set so they're interchangeable per SPEC.
	copy, err := deepcopy.TraverseStringsFunc(v, func(v string) (string, error) {
		tpl, err := template.New("").Funcs(templateFuncs).Parse(v)
		if err != nil {
			return v, err
		}
		var b bytes.Buffer
		if err := tpl.Execute(&b, data); err != nil {
			return v, err
		}
		out := strings.ReplaceAll(b.String(), "<no value>", "")
		if expandShell {
			out = ExpandShell(out, data)
		}
		return out, nil
	})
	if err != nil {
		cache.err = err
		return v
	}

	return copy
}

// ExpandShell expands shell-native variable references in s against data.
// Recognizes:
//
//   - ${NAME} — bracketed form, NAME must be [a-zA-Z_][a-zA-Z0-9_]*
//   - $NAME   — unbracketed form, NAME same charset
//   - $$      — literal $ (SPEC §Template Syntax)
//
// Unknown references are left literal so the shell can handle them — e.g.
// `$?`, `$1`, or an env-only var not declared in the Ritefile pass through
// unchanged for mvdan/sh to interpret downstream. Only refs whose name is a
// known key in data get substituted.
func ExpandShell(s string, data map[string]any) string {
	if !strings.ContainsRune(s, '$') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		c := s[i]
		if c != '$' {
			b.WriteByte(c)
			i++
			continue
		}
		// c == '$'
		if i+1 >= len(s) {
			b.WriteByte('$')
			i++
			continue
		}
		next := s[i+1]
		if next == '$' {
			// $$ → literal $
			b.WriteByte('$')
			i += 2
			continue
		}
		name, raw, advance := parseShellRef(s[i+1:])
		if name == "" {
			// Not a valid ref (e.g. `$?`, `$1`, `$-`) — keep literal.
			b.WriteByte('$')
			i++
			continue
		}
		if v, ok := data[name]; ok {
			fmt.Fprint(&b, v)
		} else {
			// Unknown var — preserve the original syntax so the shell can
			// resolve it from its own env if desired.
			b.WriteByte('$')
			b.WriteString(raw)
		}
		i += 1 + advance
	}
	return b.String()
}

// parseShellRef reads a variable reference from the start of s (which is
// assumed to sit just past a leading `$`). Returns the variable name, the raw
// source text consumed (for literal passthrough of unknown refs), and the
// number of bytes advanced in s.
func parseShellRef(s string) (name, raw string, advance int) {
	if len(s) == 0 {
		return "", "", 0
	}
	if s[0] == '{' {
		end := strings.IndexByte(s, '}')
		if end < 0 {
			return "", "", 0
		}
		n := s[1:end]
		if !isShellIdent(n) {
			return "", "", 0
		}
		return n, s[:end+1], end + 1
	}
	j := 0
	for j < len(s) && isShellIdentByte(s[j], j == 0) {
		j++
	}
	if j == 0 {
		return "", "", 0
	}
	return s[:j], s[:j], j
}

func isShellIdent(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isShellIdentByte(s[i], i == 0) {
			return false
		}
	}
	return true
}

func isShellIdentByte(c byte, first bool) bool {
	switch {
	case c == '_':
		return true
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		return true
	case !first && c >= '0' && c <= '9':
		return true
	}
	return false
}

func ReplaceGlobs(globs []*ast.Glob, cache *Cache) []*ast.Glob {
	if cache.err != nil || len(globs) == 0 {
		return nil
	}

	new := make([]*ast.Glob, len(globs))
	for i, g := range globs {
		new[i] = &ast.Glob{
			Glob:   Replace(g.Glob, cache),
			Negate: g.Negate,
		}
	}
	return new
}

func ReplaceVar(v ast.Var, cache *Cache) ast.Var {
	return ReplaceVarWithExtra(v, cache, nil)
}

func ReplaceVarWithExtra(v ast.Var, cache *Cache, extra map[string]any) ast.Var {
	if v.Ref != "" {
		return ast.Var{Value: ResolveRef(v.Ref, cache), Export: v.Export}
	}
	return ast.Var{
		Value:  ReplaceWithExtra(v.Value, cache, extra),
		Sh:     ReplaceWithExtra(v.Sh, cache, extra),
		Live:   v.Live,
		Ref:    v.Ref,
		Dir:    v.Dir,
		Export: v.Export,
	}
}

func ReplaceVars(vars *ast.Vars, cache *Cache) *ast.Vars {
	return ReplaceVarsWithExtra(vars, cache, nil)
}

func ReplaceVarsWithExtra(vars *ast.Vars, cache *Cache, extra map[string]any) *ast.Vars {
	if cache.err != nil || vars.Len() == 0 {
		return nil
	}

	newVars := ast.NewVars()
	for k, v := range vars.All() {
		newVars.Set(k, ReplaceVarWithExtra(v, cache, extra))
	}

	return newVars
}
