# Validating a Ritefile

`rite --validate` parses the Ritefile and runs the schema/version checks **without executing any task, evaluating any `sh:` var, or reading dotenv files**. It's the surface meant for editor extensions, pre-commit hooks, and CI lint stages — anywhere you want confidence that a file will run before you're willing to actually run it.

```sh
# validate the Ritefile in the current directory
rite --validate

# validate a specific file
rite --validate path/to/Ritefile.yml

# machine-readable output
rite --validate --json
```

On success the command prints `ok` to stdout and exits `0`. On failure it exits with one of the Ritefile-related [exit codes](./cli#exit-codes) and prints a human-readable error to stderr.

`--validate` is mutually exclusive with `--init`, `--migrate`, and task execution — it runs, reports, and exits.

## What validate checks

The check chain is a strict subset of the normal startup path:

1. **Root discovery** — locate the Ritefile (respecting `--taskfile` / `--dir` / the positional path argument).
2. **Parse + merge `includes:`** — full YAML decode into the Ritefile AST, and recursive resolution of the include graph. Cycle detection runs here.
3. **Schema version check** — the file must declare a supported schema version (currently `'3'`; below or at/above the next major is rejected).

What it **does not** do, on purpose:

- `sh:` dynamic vars are not evaluated. A `vars: { FOO: { sh: "exit 1" } }` declaration passes validation. This is the whole point of the subset — you get signal about *shape* without paying the cost of *execution*.
- Dotenv files are not read.
- The compiler is not constructed — no variable templating, no task graph resolution beyond what parsing requires.
- Individual tasks are not inspected for semantic coherence (e.g. whether a `deps:` entry resolves). That requires variable resolution and is out of scope for a pure static check.

If you need the full preflight including dynamic vars, run `rite --dry` instead.

## JSON output

Pair with `--json` for tooling integration. Success:

```json
{ "ok": true }
```

Failure:

```json
{
  "ok": false,
  "errors": [
    {
      "severity": "error",
      "message": "rite: Failed to parse Ritefile.yml:\nyaml: line 4: did not find expected ',' or ']'",
      "code": 104
    }
  ]
}
```

`code` is the [exit code](./cli#exit-codes) the process will return (same code routes via the process exit status), letting editor integrations map errors to the right category. When the underlying error carries file/line info (typed-shape mismatches surfaced as schema decode errors), the entry also includes `file`, `line`, and `col`.

The JSON is always emitted to stdout; human-readable error text still goes to stderr, which most CI parsers ignore. That split lets you pipe the JSON into `jq` without sanitizing around diagnostics.

## Exit codes

`--validate` reuses the existing Ritefile exit code range — no new codes were added:

| Code | Meaning |
|------|---------|
| `0`   | Valid — Ritefile parsed and schema check passed |
| `100` | Ritefile not found (no file at the path, or no Ritefile in the walked directory tree) |
| `102` | Schema decode error — key had wrong type, malformed AST shape |
| `103` | Unsupported schema version |
| `104` | Invalid Ritefile — YAML could not be parsed |
| `105` | Include cycle detected |

See the full [exit code table](./cli#exit-codes) for the context these fit into.

## CI example

```yaml
- name: Validate Ritefile
  run: rite --validate --json > validate.json || (cat validate.json && exit 1)
```

## Pre-commit hook

```sh
#!/bin/sh
# .git/hooks/pre-commit
exec rite --validate
```
