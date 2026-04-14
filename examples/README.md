# rite examples

Two subtrees, both shipped:

| Subtree | Purpose |
|---|---|
| [`recipes/`](./recipes/) | Hand-crafted pedagogical Ritefiles — one idea per directory. Read these first. |
| [`migrated/`](./migrated/) | Real-world corpus migrated from [`go-task/examples`](https://github.com/go-task/examples) via `rite --migrate`. Reference material and a migration-tool regression fence. |

`recipes/` is hand-written for clarity. `migrated/` is mechanically produced — each subdirectory links back to the exact upstream SHA it was derived from and lists any `rite migrate` warnings that fired. Upstream MIT attribution lives in [`NOTICE`](./NOTICE).

## CI smoke

The top-level `Ritefile.yml` runs `rite -l` against every `Ritefile.yml` under both subtrees — parse only, never execute. See `rite examples:verify`.
