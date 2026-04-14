# Migrated examples

Real-world Taskfiles from the upstream [`go-task/examples`](https://github.com/go-task/examples) corpus, mechanically converted with `rite --migrate`. These are not pedagogy — read [`../recipes/`](../recipes/) for that. These serve two purposes:

1. **Reference** — show what a typical migrated Ritefile looks like, including any warnings the migrator emits.
2. **Regression fence** — `rite examples:verify` parses every `Ritefile.yml` under this tree on CI, so a future change to the migrator or parser that breaks real-world files trips here first.

## Catalog

| Project | Upstream path | Warnings |
|---|---|---|
| [`go-web-app/`](./go-web-app/) | `go-web-app/Taskfile.yml` | 1 × `TEMPLATE-KEPT` (for `{{exeExt}}`) |

Each entry has its own `README.md` with a permalink to the exact upstream SHA, the migrate-tool warnings (or "clean"), and attribution.

## Upstream SHA

All files in this tree were produced from [`go-task/examples@c57278bc`](https://github.com/go-task/examples/tree/c57278bcf08acd347a0292dc18dd5672dfc1485f) — re-run against a newer upstream SHA will happen in follow-up passes.

## Attribution

Original Taskfiles © the respective upstream authors, licensed under MIT. See [`../NOTICE`](../NOTICE) for the full license text.

## Regenerating

```sh
git clone --depth 1 https://github.com/go-task/examples /tmp/upstream
for tf in $(find /tmp/upstream -name Taskfile.yml); do
  rite --migrate "$tf"
done
```

Copy the generated `Ritefile.yml` files into `examples/migrated/<project>/`, update each project's README with the new upstream SHA and any new warnings, then run `rite examples:verify` to smoke-check the tree.
