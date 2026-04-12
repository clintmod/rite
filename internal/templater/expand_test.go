package templater

import "testing"

func TestExpandShell(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"NAME":    "world",
		"PORT":    8080,
		"EMPTY":   "",
		"WITH_NS": "hello",
	}

	cases := []struct {
		in, out string
	}{
		// bracketed
		{"hello ${NAME}", "hello world"},
		{"${NAME}${NAME}", "worldworld"},
		{"${PORT}", "8080"},
		{"${EMPTY}", ""},
		{"${WITH_NS}", "hello"},
		// unbracketed
		{"hello $NAME", "hello world"},
		{"$NAME suffix", "world suffix"},
		{"$NAMEsuffix", "$NAMEsuffix"}, // NAMEsuffix is one ident, unknown → literal passthrough
		// $$ escape
		{"cost: $$5", "cost: $5"},
		{"$$NAME", "$NAME"}, // $$ collapses, leaves $NAME which shell will handle
		// unknown refs stay literal for the shell
		{"${UNKNOWN}", "${UNKNOWN}"},
		{"$UNKNOWN", "$UNKNOWN"},
		{"$?", "$?"},
		{"$1", "$1"},
		{"$-", "$-"},
		// mixed
		{"${NAME} $PORT", "world 8080"},
		{"prefix-${NAME}-$PORT-suffix", "prefix-world-8080-suffix"},
		// unclosed bracket — literal
		{"${UNCLOSED", "${UNCLOSED"},
		// empty name in braces — literal
		{"${}", "${}"},
		// no dollars — fast path
		{"no vars here", "no vars here"},
		{"", ""},
	}

	for _, c := range cases {
		if got := ExpandShell(c.in, data); got != c.out {
			t.Errorf("ExpandShell(%q) = %q, want %q", c.in, got, c.out)
		}
	}
}
