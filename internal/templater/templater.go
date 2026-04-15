package templater

import (
	"bytes"
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/go-task/template"

	"github.com/clintmod/rite/internal/deepcopy"
	"github.com/clintmod/rite/taskfile/ast"
)

// Cache is a help struct that allow us to call "replaceX" funcs multiple
// times, without having to check for error each time. The first error that
// happen will be assigned to r.err, and consecutive calls to funcs will just
// return the zero value.
//
// Safe for concurrent use: internal state is guarded by mu. Concurrent
// Replace calls from different goroutines (e.g. the output.Group wrappers
// used by parallel deps) used to race on the cacheMap lazy init — see #52.
type Cache struct {
	Vars *ast.Vars

	mu       sync.Mutex
	cacheMap map[string]any
	err      error
}

// ensureCacheMapLocked initializes cacheMap on first use. Caller must hold mu.
func (r *Cache) ensureCacheMapLocked() {
	if r.cacheMap != nil {
		return
	}
	if r.Vars != nil {
		r.cacheMap = r.Vars.ToCacheMap()
	} else {
		r.cacheMap = make(map[string]any)
	}
}

func (r *Cache) ResetCache() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cacheMap = r.Vars.ToCacheMap()
}

// Seed adds fallback entries to the templater's view that aren't part of the
// underlying Vars. Existing keys are preserved. Used to make built-in vars
// visible to template resolution while keeping them out of the canonical
// variable set until lowest-priority merge time.
func (r *Cache) Seed(m map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureCacheMapLocked()
	for k, v := range m {
		if _, exists := r.cacheMap[k]; !exists {
			r.cacheMap[k] = v
		}
	}
}

// Update records a key/value pair that was just added to the underlying Vars
// so the templater's view stays consistent without rebuilding from scratch.
func (r *Cache) Update(k string, v any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureCacheMapLocked()
	r.cacheMap[k] = v
}

func (r *Cache) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

// snapshot returns an independent copy of cacheMap and the sticky error, both
// read under the lock so concurrent callers never observe a half-initialized
// or mid-mutation map. Callers operate on the returned clone without holding
// the lock — template execution can be slow and there's no reason to
// serialize it across goroutines sharing a Cache.
func (r *Cache) snapshot() (map[string]any, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return nil, r.err
	}
	r.ensureCacheMapLocked()
	return maps.Clone(r.cacheMap), nil
}

// setErr records the first error seen by any replace*/Resolve* call. No-op
// if err is nil or a prior error is already latched.
func (r *Cache) setErr(err error) {
	if err == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err == nil {
		r.err = err
	}
}

func ResolveRef(ref string, cache *Cache) any {
	data, err := cache.snapshot()
	if err != nil {
		return nil
	}

	if ref == "." {
		return data
	}
	t, err := template.New("resolver").Funcs(templateFuncs).Parse(fmt.Sprintf("{{%s}}", ref))
	if err != nil {
		cache.setErr(err)
		return nil
	}
	val, err := t.Resolve(data)
	if err != nil {
		cache.setErr(err)
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
	// snapshot returns a locked clone of cacheMap; the returned map is the
	// goroutine's private copy, so the subsequent merge + template execution
	// don't have to hold the Cache lock. The sticky-error short-circuit lives
	// inside snapshot so a prior latched err causes an early bail here too.
	data, err := cache.snapshot()
	if err != nil {
		return v
	}
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
		cache.setErr(err)
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
//
// Quoting honors POSIX shell semantics: `'…'` suppresses expansion entirely,
// `"…"` keeps expanding, `\$` outside or inside double quotes is a literal
// `$`. Heredocs participate too — `<<'DELIM'` or `<<\DELIM` disables
// expansion in the body; bare `<<DELIM` keeps it. Single-quoted strings
// treat `\` as a literal character (POSIX). This exists so Ritefiles can
// emit literal `$X` runs (heredoc help text, sed scripts, etc.) without
// sentinel workarounds — see #121.
func ExpandShell(s string, data map[string]any) string {
	if !strings.ContainsRune(s, '$') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))

	const (
		qNone = iota
		qSingle
		qDouble
	)
	state := qNone

	var pending []heredocState // declared on current line, body starts at next \n
	var active *heredocState   // currently inside a heredoc body

	i := 0
	for i < len(s) {
		c := s[i]

		if active != nil {
			// At line start (i==0 or just past a \n), check end-of-body.
			atLineStart := i == 0 || s[i-1] == '\n'
			if atLineStart {
				lineStart := i
				if active.stripTab {
					for lineStart < len(s) && s[lineStart] == '\t' {
						lineStart++
					}
				}
				lineEnd := lineStart
				for lineEnd < len(s) && s[lineEnd] != '\n' {
					lineEnd++
				}
				if s[lineStart:lineEnd] == active.delim {
					b.WriteString(s[i:lineEnd])
					i = lineEnd
					active = nil
					continue
				}
			}
			if active.expand {
				if c == '\\' && i+1 < len(s) && s[i+1] == '$' {
					b.WriteByte('$')
					i += 2
					continue
				}
				if c == '$' {
					adv, ok := writeDollar(&b, s, i, data)
					if ok {
						i += adv
						continue
					}
				}
			}
			b.WriteByte(c)
			i++
			continue
		}

		switch state {
		case qSingle:
			b.WriteByte(c)
			i++
			if c == '\'' {
				state = qNone
			}
			continue

		case qDouble:
			if c == '"' {
				b.WriteByte(c)
				i++
				state = qNone
				continue
			}
			if c == '\\' && i+1 < len(s) && s[i+1] == '$' {
				// POSIX: \$ in "..." is a literal $.
				b.WriteByte('$')
				i += 2
				continue
			}
			if c == '$' {
				adv, ok := writeDollar(&b, s, i, data)
				if ok {
					i += adv
					continue
				}
			}
			b.WriteByte(c)
			i++
			continue
		}

		// state == qNone, no active heredoc.
		if c == '\n' && len(pending) > 0 {
			b.WriteByte('\n')
			h := pending[0]
			pending = pending[1:]
			active = &h
			i++
			continue
		}
		if c == '\'' {
			b.WriteByte(c)
			i++
			state = qSingle
			continue
		}
		if c == '"' {
			b.WriteByte(c)
			i++
			state = qDouble
			continue
		}
		if c == '\\' && i+1 < len(s) && s[i+1] == '$' {
			// POSIX: \$ outside quotes is a literal $.
			b.WriteByte('$')
			i += 2
			continue
		}
		if c == '<' && i+1 < len(s) && s[i+1] == '<' {
			if h, end, ok := parseHeredocStart(s, i); ok {
				b.WriteString(s[i:end])
				pending = append(pending, h)
				i = end
				continue
			}
		}
		if c == '$' {
			adv, ok := writeDollar(&b, s, i, data)
			if ok {
				i += adv
				continue
			}
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

// writeDollar handles the `$…` substitution at s[i]. Returns the number of
// bytes consumed and whether anything was written. If !ok, the caller should
// fall through to the default literal-byte branch.
func writeDollar(b *strings.Builder, s string, i int, data map[string]any) (int, bool) {
	if i+1 >= len(s) {
		b.WriteByte('$')
		return 1, true
	}
	if s[i+1] == '$' {
		b.WriteByte('$')
		return 2, true
	}
	name, raw, advance := parseShellRef(s[i+1:])
	if name == "" {
		b.WriteByte('$')
		return 1, true
	}
	if v, ok := data[name]; ok {
		fmt.Fprint(b, v)
	} else {
		b.WriteByte('$')
		b.WriteString(raw)
	}
	return 1 + advance, true
}

// heredocState carries the parsed shape of a `<<DELIM` declaration so
// ExpandShell can decide whether to keep expanding inside the body.
type heredocState struct {
	delim    string
	expand   bool
	stripTab bool
}

// parseHeredocStart inspects s starting at the `<<` operator at position i.
// Returns the heredoc state, the index just past the delimiter token, and ok
// if a well-formed heredoc declaration was found.
func parseHeredocStart(s string, i int) (h heredocState, end int, ok bool) {
	j := i + 2
	if j < len(s) && s[j] == '-' {
		h.stripTab = true
		j++
	}
	for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
		j++
	}
	if j >= len(s) {
		return h, 0, false
	}
	switch s[j] {
	case '\'':
		k := strings.IndexByte(s[j+1:], '\'')
		if k < 0 {
			return h, 0, false
		}
		h.delim = s[j+1 : j+1+k]
		h.expand = false
		end = j + 1 + k + 1
	case '"':
		k := strings.IndexByte(s[j+1:], '"')
		if k < 0 {
			return h, 0, false
		}
		h.delim = s[j+1 : j+1+k]
		h.expand = false
		end = j + 1 + k + 1
	case '\\':
		k := j + 1
		for k < len(s) && isHeredocDelimByte(s[k]) {
			k++
		}
		if k == j+1 {
			return h, 0, false
		}
		h.delim = s[j+1 : k]
		h.expand = false
		end = k
	default:
		k := j
		for k < len(s) && isHeredocDelimByte(s[k]) {
			k++
		}
		if k == j {
			return h, 0, false
		}
		h.delim = s[j:k]
		h.expand = true
		end = k
	}
	if h.delim == "" {
		return h, 0, false
	}
	return h, end, true
}

func isHeredocDelimByte(c byte) bool {
	switch {
	case c == '_', c == '-', c == '.':
		return true
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		return true
	case c >= '0' && c <= '9':
		return true
	}
	return false
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
