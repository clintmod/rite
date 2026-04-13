# Where rite finds your Ritefile

Three modes for telling rite which file to run: implicit (walk up the tree), explicit (`-t` flag), or unusual (global, stdin).

## Implicit: walk up the tree

`rite <task>` with no `-t` flag looks for a Ritefile starting in the current directory, then walks **upward** through parents until it finds one or hits the filesystem root. This is what you want 99% of the time — you can run rite from anywhere inside a project and it finds the project's Ritefile.

```sh
$ pwd
/Users/clint/projects/myapp/internal/auth
$ rite test           # finds /Users/clint/projects/myapp/Ritefile.yml
```

The walk stops if the *directory owner* changes — protection against running someone else's Ritefile when you `cd` into another user's home tree on a multi-tenant box.

### Filenames recognized

In each directory, in this order:

| Filename | Notes |
|---|---|
| `Ritefile` | No extension |
| `Ritefile.yml` | Most common |
| `Ritefile.yaml` | Same, with the long extension |
| `Ritefile.dist.yml` | Project-shipped default; commit this if you want a "fallback if nobody made a local one" |
| `Ritefile.dist.yaml` | Same, long extension |

Lowercase variants (`ritefile`, `ritefile.yml`, etc.) are also accepted. The `.dist` form is checked *after* the non-dist form in each directory — so a developer-local `Ritefile.yml` overrides the committed `Ritefile.dist.yml` without anyone touching the dist file.

## Explicit: `-t FILE` / `--taskfile FILE`

Point rite at a specific file:

```sh
rite -t ./build/Ritefile.yml test
rite --taskfile /etc/myapp/Ritefile.yml deploy
```

Disables the walk-up behavior — you get exactly the file you asked for, no parent search.

## Explicit: `-d DIR` / `--dir DIR`

Run rite as if it had been invoked from a different directory. Walk-up still applies, just from the new starting point:

```sh
rite -d ~/code/myapp test
```

The cmds run with their own `dir:` resolution (relative to the Ritefile, not your shell CWD). See [`dir:` in tasks](/dir).

## Global Ritefile: `-g` / `--global`

For tasks you want available everywhere on a machine — system maintenance scripts, cross-project shortcuts — keep a Ritefile in `$HOME` and invoke with `-g`:

```sh
rite -g cleanup       # runs $HOME/Ritefile.yml task `cleanup`
```

The `-g` flag changes the search root to `$HOME` and skips the walk-up. Common pattern: `~/Ritefile.yml` holds personal aliases (`brew-update`, `k8s-context`, `vpn-toggle`), invoked from anywhere with `rite -g <task>`.

`-g` and `-d` are mutually exclusive — you can't say both "use a different start dir" and "use the home dir."

> **Sharp edge:** the upstream `--global` help string still mentions "Taskfile.{yml,yaml}" because the field name wasn't rebranded. Functionally it walks `$HOME` looking for *any* recognized rite filename including `Ritefile.yml`. If you have both `Taskfile.yml` and `Ritefile.yml` in `$HOME`, current behavior is upstream-shaped — favor renaming the file to `Ritefile.yml` to avoid ambiguity.

## Reading from stdin: `-t -`

A dash for the `-t` argument reads the Ritefile from stdin:

```sh
cat ./generated.yml | rite -t - <task>
```

Or in a heredoc:

```sh
rite -t - <<'EOF'
version: '3'
tasks:
  hello: echo "hi from stdin"
EOF
```

```sh
rite -t - hello
```

The "file location" in error messages renders as `__stdin__`. Useful for:

- Generating a Ritefile programmatically (e.g., from a template) and piping it in without writing to disk.
- Testing Ritefile snippets without committing a fixture.
- One-shot CI tasks where the steps come from a different config system.

Stdin Ritefiles can use `includes:` only with absolute paths or `-d` — they have no implicit "directory" to resolve relative paths against.

## What rite does NOT do (vs. some alternatives)

- **No automatic OS-suffix files.** rite does not silently merge `Ritefile_darwin.yml` when running on macOS. If you want that pattern, declare it explicitly via [includes with templated paths](/includes#os-specific-includes) — `includes: { local: ./Ritefile_${OS}.yml }`.
- **No XDG search.** rite doesn't look in `$XDG_CONFIG_HOME/rite/` for anything. The global Ritefile lives in `$HOME` (matching upstream); use `-g` to invoke it.
- **No remote-by-default.** Remote Ritefile loading exists behind the experiment flag `RITE_X_REMOTE_TASKFILES=1` but is not production-grade. Don't rely on it for shared CI Ritefiles yet.
