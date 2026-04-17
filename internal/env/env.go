package env

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/clintmod/rite/taskfile/ast"
)

const riteVarPrefix = "RITE_"

// GetEnviron the all return all environment variables encapsulated on a
// ast.Vars
func GetEnviron() *ast.Vars {
	m := ast.NewVars()
	for _, e := range os.Environ() {
		keyVal := strings.SplitN(e, "=", 2)
		key, val := keyVal[0], keyVal[1]
		m.Set(key, ast.Var{Value: val})
	}
	return m
}

func Get(t *ast.Task) []string {
	// SPEC §vars/env Unification: every declared variable exports to the
	// process environment unless marked export: false. We union t.Vars and
	// t.Env here so a user's top-level `vars:` block reaches the cmd shell
	// just like the `env:` block does. env: values override vars: values
	// on name conflict — the env block remains the explicit way to say
	// "this is for export," while vars: is the default "export unless
	// export: false" surface.
	combined := ast.NewVars()
	if t.Vars != nil {
		combined.Merge(t.Vars, nil)
	}
	if t.Env != nil {
		combined.Merge(t.Env, nil)
	}
	if combined.Len() == 0 {
		return nil
	}
	return GetFromVars(combined)
}

func GetFromVars(env *ast.Vars) []string {
	environ := os.Environ()

	// Walk the underlying Vars so we can honor per-var Export metadata —
	// ToCacheMap strips the ast.Var wrapper. SPEC §vars/env Unification
	// lets users mark a var as `export: false` to keep it out of the cmd
	// process environ while still being visible to Ritefile templating.
	for k, v := range env.All() {
		if !v.Exported() {
			continue
		}
		// Match ToCacheMap's skip-for-unresolved-dynamic rule.
		if v.Sh != nil && *v.Sh != "" && v.Value == nil {
			continue
		}
		var value any
		if v.Live != nil {
			value = v.Live
		} else {
			value = v.Value
		}
		if !isTypeAllowed(value) {
			continue
		}
		// Shell env always wins per SPEC §Variable Precedence tier 1 —
		// "never overridden by anything in a Ritefile." The SPEC admits
		// no opt-out, so this is unconditional.
		if _, alreadySet := os.LookupEnv(k); alreadySet {
			continue
		}
		environ = append(environ, fmt.Sprintf("%s=%v", k, value))
	}

	return environ
}

func isTypeAllowed(v any) bool {
	switch v.(type) {
	case string, bool, int, float32, float64:
		return true
	default:
		return false
	}
}

// ForwardColor appends the de-facto-standard "force color on" env vars
// (CLICOLOR_FORCE=1 and FORCE_COLOR=1) to the given child environ slice when
// rite's own color detection has resolved to on (issue #153).
//
// Motivation: when a rite task's `cmds:` spawn a color-aware child (another
// `rite -l`, `ls`, `rg`, `fd`, ...) the child sees its stdout is a pipe to
// rite's output machinery and strips color. The outer rite is still writing
// to a real TTY, so the bytes the child would have emitted are exactly what
// the user would see. Forwarding the well-known CLICOLOR_FORCE / FORCE_COLOR
// signals lets modern CLIs opt into color even over pipes.
//
// Honored user overrides (no injection when any of these hold in the parent
// environ, regardless of colorOn):
//   - NO_COLOR is set to any non-empty value (no-color.org convention).
//   - CLICOLOR_FORCE is explicitly set to "0".
//   - FORCE_COLOR is explicitly set to "0".
//
// The passed-in environ takes precedence when the key is present there;
// otherwise the process's own os.Environ() is consulted, because execext
// falls back to os.Environ() when the cmd is spawned with an empty env
// slice.
//
// NO_COLOR itself is never touched here — it passes through as the user set
// it and the child can continue to honor it.
func ForwardColor(environ []string, colorOn bool) []string {
	if !colorOn {
		return environ
	}
	// Walk the caller-provided environ first so explicit values there win.
	var sawNoColor, sawCCF, sawFC bool
	for _, kv := range environ {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		key, val := kv[:eq], kv[eq+1:]
		switch key {
		case "NO_COLOR":
			sawNoColor = true
			if val != "" {
				return environ
			}
		case "CLICOLOR_FORCE":
			sawCCF = true
			if val == "0" {
				return environ
			}
		case "FORCE_COLOR":
			sawFC = true
			if val == "0" {
				return environ
			}
		}
	}
	// Fall back to os.Environ for keys not present in the caller's slice
	// (execext resolves an empty cmd env to os.Environ() at spawn time, so
	// those values will still reach the child).
	if !sawNoColor {
		if v, ok := os.LookupEnv("NO_COLOR"); ok && v != "" {
			return environ
		}
	}
	if !sawCCF {
		if v, ok := os.LookupEnv("CLICOLOR_FORCE"); ok && v == "0" {
			return environ
		}
	}
	if !sawFC {
		if v, ok := os.LookupEnv("FORCE_COLOR"); ok && v == "0" {
			return environ
		}
	}
	return append(environ, "CLICOLOR_FORCE=1", "FORCE_COLOR=1")
}

// GetRiteEnv returns the value of a RITE_-prefixed environment variable.
// The prefix is applied automatically; callers pass the bare suffix.
func GetRiteEnv(key string) string {
	return os.Getenv(riteVarPrefix + key)
}

// GetRiteEnvBool returns the boolean value of a RITE_-prefixed env var.
// Returns the value and true if set and valid, or false and false if not set or invalid.
func GetRiteEnvBool(key string) (bool, bool) {
	v := GetRiteEnv(key)
	if v == "" {
		return false, false
	}
	b, err := strconv.ParseBool(v)
	return b, err == nil
}

// GetRiteEnvInt returns the integer value of a RITE_-prefixed env var.
// Returns the value and true if set and valid, or 0 and false if not set or invalid.
func GetRiteEnvInt(key string) (int, bool) {
	v := GetRiteEnv(key)
	if v == "" {
		return 0, false
	}
	i, err := strconv.Atoi(v)
	return i, err == nil
}

// GetRiteEnvDuration returns the duration value of a RITE_-prefixed env var.
// Returns the value and true if set and valid, or 0 and false if not set or invalid.
func GetRiteEnvDuration(key string) (time.Duration, bool) {
	v := GetRiteEnv(key)
	if v == "" {
		return 0, false
	}
	d, err := time.ParseDuration(v)
	return d, err == nil
}

// GetRiteEnvString returns the string value of a RITE_-prefixed env var.
// Returns the value and true if set (non-empty), or empty string and false if not set.
func GetRiteEnvString(key string) (string, bool) {
	v := GetRiteEnv(key)
	return v, v != ""
}

// GetRiteEnvStringSlice returns a comma-separated list from a RITE_-prefixed env var.
// Returns the slice and true if set (non-empty), or nil and false if not set.
func GetRiteEnvStringSlice(key string) ([]string, bool) {
	v := GetRiteEnv(key)
	if v == "" {
		return nil, false
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return nil, false
	}
	return result, true
}
