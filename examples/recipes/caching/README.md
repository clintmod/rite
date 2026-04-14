# caching

`sources:` + `generates:` + `method:` make a task skip when its inputs haven't changed.

## Run

```sh
cd examples/recipes/caching

rite                       # builds build/index.out
rite                       # → rite: Task "default" is up to date   (skipped)
echo gamma > inputs/c.txt
rite                       # rebuilds — source set changed

rite timestamp             # same output, but uses mtime comparison
rite none                  # always runs, regardless of fingerprint
rite clean                 # wipe build/ and .rite/
```

## What to notice

- **`method: checksum`** (the default) fingerprints the source globs *and* the cmd strings. Robust against mtime jitter (CI caches, `touch`); the tradeoff is per-run IO to compute the hash.
- **`method: timestamp`** compares `max(source mtime)` against `min(generates mtime)`. Cheaper; brittle if something rewrites a source file with unchanged content.
- **`method: none`** opts out entirely — useful for commands that must execute every time (e.g., `git pull`, `docker compose up`) even with `sources:`/`generates:` declared for documentation.
- **State lives in `.rite/`**. Delete it (or the `generates:` outputs) to force a rebuild. Commit `.rite/` to your `.gitignore`.

Tip: `rite --dry` shows what *would* run without side-effects. Useful when debugging why a task is or isn't re-running.

## See also

- [Incremental builds — sources and generates](https://clintmod.github.io/rite/sources-and-generates)
- [Run modes](https://clintmod.github.io/rite/run-modes)
