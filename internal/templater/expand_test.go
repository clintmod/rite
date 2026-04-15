package templater

import "testing"

func TestExpandShell(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"NAME":    "world",
		"PORT":    8080,
		"EMPTY":   "",
		"WITH_NS": "hello",
		"A":       "alpha",
		"B":       "beta",
		"C":       "gamma",
		"D":       "delta",
		"X":       "xray",
	}

	cases := []struct {
		name    string
		in, out string
	}{
		// --- bracketed ---
		{"bracketed simple", "hello ${NAME}", "hello world"},
		{"bracketed adjacent", "${NAME}${NAME}", "worldworld"},
		{"bracketed numeric value", "${PORT}", "8080"},
		{"bracketed empty value", "${EMPTY}", ""},
		{"bracketed with underscore", "${WITH_NS}", "hello"},
		// --- unbracketed ---
		{"bare simple", "hello $NAME", "hello world"},
		{"bare suffix space", "$NAME suffix", "world suffix"},
		{"bare extends ident", "$NAMEsuffix", "$NAMEsuffix"}, // one ident, unknown → literal passthrough
		// --- $$ escape ---
		{"dollar dollar digit", "cost: $$5", "cost: $5"},
		{"dollar dollar name", "$$NAME", "$NAME"}, // $$ collapses, leaves $NAME which shell will handle
		// --- unknown refs stay literal for the shell ---
		{"unknown bracketed", "${UNKNOWN}", "${UNKNOWN}"},
		{"unknown bare", "$UNKNOWN", "$UNKNOWN"},
		{"positional ?", "$?", "$?"},
		{"positional 1", "$1", "$1"},
		{"flags -", "$-", "$-"},
		// --- mixed ---
		{"mixed forms", "${NAME} $PORT", "world 8080"},
		{"interpolated word", "prefix-${NAME}-$PORT-suffix", "prefix-world-8080-suffix"},
		// --- malformed ---
		{"unclosed bracket", "${UNCLOSED", "${UNCLOSED"},
		{"empty braces", "${}", "${}"},
		// --- fast paths ---
		{"no dollars", "no vars here", "no vars here"},
		{"empty input", "", ""},

		// --- POSIX quoting (issue #121) ---
		// Single quotes suppress expansion entirely.
		{"single suppresses bracketed", "'${NAME}'", "'${NAME}'"},
		{"single suppresses bare", "'$NAME'", "'$NAME'"},
		{"single literal slash dollar", `'\$X'`, `'\$X'`},
		{"single literal positional", "'$1'", "'$1'"},
		// Double quotes keep expanding.
		{"double expands bracketed", `"${NAME}"`, `"world"`},
		{"double expands bare", `"$NAME"`, `"world"`},
		{"double slash dollar literal", `"\${NAME}"`, `"${NAME}"`},
		{"double slash dollar literal bare", `"\$NAME"`, `"$NAME"`},
		// Backslash escape outside quotes.
		{"backslash dollar outside", `\${NAME}`, `${NAME}`},
		{"backslash bare dollar outside", `\$NAME`, `$NAME`},
		// Mixed in one line — each region honors its own rule.
		{
			"mixed quoting per region",
			`echo '${A}' "${B}" \${C} ${D}`,
			`echo '${A}' "beta" ${C} delta`,
		},
		// Nested: outer wins.
		{"double containing single content", `"'${X}'"`, `"'xray'"`},
		{"single containing double content", `'"${X}"'`, `'"${X}"'`},
		// Quotes mid-token close & re-open state.
		{"quote toggles state", `${A}'${B}'${C}`, `alpha'${B}'gamma`},
		// Heredoc — quoted delimiter suppresses; bare expands.
		{
			"heredoc quoted single",
			"cat <<'EOF'\n${NAME}\nEOF",
			"cat <<'EOF'\n${NAME}\nEOF",
		},
		{
			"heredoc quoted double",
			"cat <<\"EOF\"\n${NAME}\nEOF",
			"cat <<\"EOF\"\n${NAME}\nEOF",
		},
		{
			"heredoc backslash delim",
			"cat <<\\EOF\n${NAME}\nEOF",
			"cat <<\\EOF\n${NAME}\nEOF",
		},
		{
			"heredoc bare expands",
			"cat <<EOF\n${NAME}\nEOF",
			"cat <<EOF\nworld\nEOF",
		},
		{
			"heredoc bare honors backslash dollar",
			"cat <<EOF\n\\${NAME}\nEOF",
			"cat <<EOF\n${NAME}\nEOF",
		},
		{
			"heredoc dash quoted",
			"cat <<-'EOF'\n\t${NAME}\n\tEOF",
			"cat <<-'EOF'\n\t${NAME}\n\tEOF",
		},
		{
			"heredoc dash bare",
			"cat <<-EOF\n\t${NAME}\n\tEOF",
			"cat <<-EOF\n\tworld\n\tEOF",
		},
		{
			"text after heredoc body",
			"cat <<'EOF'\n${A}\nEOF\nafter ${B}",
			"cat <<'EOF'\n${A}\nEOF\nafter beta",
		},
		// Heredoc body end matched line-exact, so a partial-prefix line is body content.
		{
			"heredoc body line containing delim word is not end",
			"cat <<EOF\nEOFISH\n${NAME}\nEOF",
			"cat <<EOF\nEOFISH\nworld\nEOF",
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := ExpandShell(c.in, data); got != c.out {
				t.Errorf("ExpandShell(%q) = %q, want %q", c.in, got, c.out)
			}
		})
	}
}
