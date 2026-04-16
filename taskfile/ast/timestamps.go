package ast

import (
	"go.yaml.in/yaml/v3"

	"github.com/clintmod/rite/errors"
)

// DefaultTimestampLayout is the Go time layout used when `timestamps: true`
// is set (at any scope). ISO 8601 extended, UTC, zero-padded millisecond
// precision, square-bracketed so the prefix is visually separable from cmd
// content. See SPEC §Output Timestamps.
const DefaultTimestampLayout = "[2006-01-02T15:04:05.000Z]"

// TimestampMarkerEnvVar is the marker variable rite injects into a cmd's
// environ whenever its output is being wrapped by a TimestampWriter. A
// nested rite invocation that sees this variable set to "1" skips its own
// timestamp wrapping entirely — otherwise every level of nesting would add
// another prefix and multi-level `rite`-calls-`rite` chains would emit
// `[ts] [ts] [ts] line`. See SPEC §Output Timestamps / "Nested invocations".
const TimestampMarkerEnvVar = "RITE_TIMESTAMPS_HANDLED"

// Timestamps is the tri-state value for the `timestamps:` knob at both the
// entrypoint level and the task level.
//
// - Unset (nil) → inherit from the next-higher scope (or off if top-level).
// - Explicit `true` → use DefaultTimestampLayout.
// - Explicit `false` → off (overrides a higher-scope `true`).
// - Explicit strftime string → use that format.
//
// The value is stored on a pointer so the YAML decoder can distinguish
// "omitted" (nil) from "false" (present, disabled). A bool-only field cannot
// express the override-off-a-global-on case.
type Timestamps struct {
	// Enabled is non-nil when the user explicitly set a value at this scope.
	// *Enabled == true means on; *Enabled == false means off.
	Enabled *bool
	// Format is the optional strftime-style format string. Empty means "use
	// the default layout" (when Enabled is *true).
	Format string
}

// IsSet reports whether the user declared a value for this scope.
func (t *Timestamps) IsSet() bool {
	if t == nil {
		return false
	}
	return t.Enabled != nil
}

// On reports whether timestamps are enabled at this scope. Callers must check
// IsSet() first — On() on an unset value returns false.
func (t *Timestamps) On() bool {
	if t == nil || t.Enabled == nil {
		return false
	}
	return *t.Enabled
}

// UnmarshalYAML accepts `true`, `false`, or a strftime string.
func (t *Timestamps) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return errors.NewRitefileDecodeError(nil, node).WithTypeMessage("timestamps")
	}
	// Try bool first. yaml.v3 tags scalars; a bare true/false has tag
	// !!bool, a quoted string has tag !!str. We accept either shape and fall
	// back to the raw value for strftime formats.
	switch node.Tag {
	case "!!bool":
		var b bool
		if err := node.Decode(&b); err != nil {
			return errors.NewRitefileDecodeError(err, node)
		}
		t.Enabled = &b
		t.Format = ""
		return nil
	case "!!str", "":
		s := node.Value
		// A quoted "true"/"false" is still a bool semantically. Users who
		// want a format literally named "true" can prefix a space; nobody
		// wants that.
		switch s {
		case "true":
			on := true
			t.Enabled = &on
			t.Format = ""
			return nil
		case "false":
			off := false
			t.Enabled = &off
			t.Format = ""
			return nil
		case "":
			return errors.NewRitefileDecodeError(nil, node).WithMessage("timestamps: empty string is not a valid format")
		}
		on := true
		t.Enabled = &on
		t.Format = s
		return nil
	}
	return errors.NewRitefileDecodeError(nil, node).WithTypeMessage("timestamps")
}

// DeepCopy returns a deep copy of the Timestamps value, including a fresh
// pointer for Enabled so callers can mutate one copy without racing the
// other.
func (t *Timestamps) DeepCopy() *Timestamps {
	if t == nil {
		return nil
	}
	c := &Timestamps{Format: t.Format}
	if t.Enabled != nil {
		v := *t.Enabled
		c.Enabled = &v
	}
	return c
}
