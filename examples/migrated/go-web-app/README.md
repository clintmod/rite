# go-web-app (migrated)

Mechanically migrated from the upstream `go-task/examples` corpus.

- **Upstream source:** [github.com/go-task/examples/tree/c57278bc/go-web-app](https://github.com/go-task/examples/tree/c57278bcf08acd347a0292dc18dd5672dfc1485f/go-web-app)
- **Migration tool:** `rite --migrate <path-to-Taskfile.yml>`
- **Migrated with rite:** v1.0.1 (+d0e32d4)

## Migrate warnings

One warning fired on this Taskfile:

```
rite migrate: TEMPLATE-KEPT go-web-app/Taskfile.yml:11:
  kept Go-template syntax "{{exeExt}}" — no equivalent ${VAR} form;
  review manually.
```

`{{exeExt}}` is a go-task-specific template function that expands to `.exe` on Windows and `""` elsewhere. rite doesn't ship a Sprig-style function table; the idiomatic rite replacement is platform-gated cmds or a dynamic var set via `sh:`. For corpus-fidelity we left the template string intact — users migrating a real codebase can replace it per the [migration guide](https://clintmod.github.io/rite/migration).

## Assets (not committed)

This directory only holds the migrated `Ritefile.yml` and this README. The Go source, templates, and frontend assets live upstream — clone `go-task/examples` at the pinned SHA above if you want to actually run the build:

```sh
git clone --depth 1 https://github.com/go-task/examples /tmp/upstream
cp Ritefile.yml /tmp/upstream/go-web-app/
cd /tmp/upstream/go-web-app && rite
```

## Attribution

Original Taskfile.yml © 2017 Task contributors. Licensed under MIT. See [`examples/NOTICE`](../../NOTICE) at the repository root.
