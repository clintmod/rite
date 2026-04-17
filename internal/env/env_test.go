package env

import (
	"slices"
	"testing"
)

// TestForwardColor_OffDoesNothing — when the outer rite's color is resolved
// as off (pipe + no force signals, or explicit --color=false), we must not
// inject CLICOLOR_FORCE / FORCE_COLOR. The child's own TTY detection is the
// right default.
func TestForwardColor_OffDoesNothing(t *testing.T) {
	t.Parallel()
	in := []string{"FOO=bar"}
	got := ForwardColor(in, false)
	if !slices.Equal(got, in) {
		t.Fatalf("expected no injection when colorOn=false, got %v", got)
	}
}

// TestForwardColor_OnInjectsBothVars — when color is on and no user override
// is set, inject both the CLICOLOR_FORCE and FORCE_COLOR de-facto standards
// so fatih/color, Node-ecosystem tools, ripgrep, fd, etc. all light up.
func TestForwardColor_OnInjectsBothVars(t *testing.T) {
	// Clear any ambient force/off signals in os.Environ so the test
	// exercises the "nothing set" branch deterministically.
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("FORCE_COLOR", "")
	got := ForwardColor([]string{"FOO=bar"}, true)
	if !slices.Contains(got, "CLICOLOR_FORCE=1") {
		t.Errorf("expected CLICOLOR_FORCE=1 in %v", got)
	}
	if !slices.Contains(got, "FORCE_COLOR=1") {
		t.Errorf("expected FORCE_COLOR=1 in %v", got)
	}
}

// TestForwardColor_NoColorInEnvironWins — a non-empty NO_COLOR in the
// caller's environ slice must suppress injection, per no-color.org. We do
// not touch NO_COLOR itself; it passes through as the user set it.
func TestForwardColor_NoColorInEnvironWins(t *testing.T) {
	// Can't t.Parallel: ForwardColor falls back to os.LookupEnv when a key
	// isn't in the passed slice, so a sibling parallel test that sets
	// NO_COLOR="" would race with this one. We constrain the input slice
	// to cover the "in slice wins over process env" branch deterministically.
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("FORCE_COLOR", "")
	in := []string{"NO_COLOR=1"}
	got := ForwardColor(in, true)
	if slices.Contains(got, "CLICOLOR_FORCE=1") || slices.Contains(got, "FORCE_COLOR=1") {
		t.Errorf("NO_COLOR=1 should suppress injection, got %v", got)
	}
}

// TestForwardColor_NoColorInProcessEnvWins — when a caller passes a nil or
// empty cmd-env slice, execext falls back to os.Environ(). We must consult
// the process env for the off-signals in that case so the child still sees
// a consistent story.
func TestForwardColor_NoColorInProcessEnvWins(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("FORCE_COLOR", "")
	got := ForwardColor(nil, true)
	if slices.Contains(got, "CLICOLOR_FORCE=1") || slices.Contains(got, "FORCE_COLOR=1") {
		t.Errorf("NO_COLOR in os.Environ should suppress injection, got %v", got)
	}
}

// TestForwardColor_NoColorEmptyDoesNotSuppress — NO_COLOR="" (explicitly
// empty) is per-spec "not set," so it should NOT suppress color. This is
// the shape flags.go uses when clearing the ambient NO_COLOR for internal
// tests, and also matches `export NO_COLOR=` (unsetting via empty).
func TestForwardColor_NoColorEmptyDoesNotSuppress(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("FORCE_COLOR", "")
	in := []string{"NO_COLOR="}
	got := ForwardColor(in, true)
	if !slices.Contains(got, "CLICOLOR_FORCE=1") {
		t.Errorf("empty NO_COLOR should not suppress, got %v", got)
	}
}

// TestForwardColor_ExplicitForceZeroWins — user explicitly sets
// CLICOLOR_FORCE=0 or FORCE_COLOR=0 to turn off even when a parent wants
// color. Honor that.
func TestForwardColor_ExplicitForceZeroWins(t *testing.T) {
	cases := []struct {
		name string
		in   []string
	}{
		{"CLICOLOR_FORCE=0 in slice", []string{"CLICOLOR_FORCE=0"}},
		{"FORCE_COLOR=0 in slice", []string{"FORCE_COLOR=0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", "")
			t.Setenv("CLICOLOR_FORCE", "")
			t.Setenv("FORCE_COLOR", "")
			got := ForwardColor(tc.in, true)
			if slices.Contains(got, "CLICOLOR_FORCE=1") {
				t.Errorf("explicit =0 should suppress CLICOLOR_FORCE=1, got %v", got)
			}
			if slices.Contains(got, "FORCE_COLOR=1") {
				t.Errorf("explicit =0 should suppress FORCE_COLOR=1, got %v", got)
			}
		})
	}
}

// TestForwardColor_ForceZeroInProcessEnvWins — same off-signal honored when
// it's only in os.Environ() (caller passed empty slice).
func TestForwardColor_ForceZeroInProcessEnvWins(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "0")
	t.Setenv("FORCE_COLOR", "")
	got := ForwardColor(nil, true)
	if slices.Contains(got, "CLICOLOR_FORCE=1") || slices.Contains(got, "FORCE_COLOR=1") {
		t.Errorf("CLICOLOR_FORCE=0 in os.Environ should suppress, got %v", got)
	}
}

// TestForwardColor_DoesNotMutateInput — ForwardColor must not mutate the
// caller's slice (append-only semantics with a defensive return). Callers
// often reuse their env slice for multiple cmds.
func TestForwardColor_DoesNotMutateInput(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("FORCE_COLOR", "")
	in := []string{"FOO=bar"}
	_ = ForwardColor(in, true)
	if len(in) != 1 || in[0] != "FOO=bar" {
		t.Errorf("ForwardColor mutated caller slice: %v", in)
	}
}
