# rite examples

Two subtrees are planned here:

| Subtree | Status | Purpose |
|---|---|---|
| [`recipes/`](./recipes/) | shipped | Hand-crafted pedagogical Ritefiles — one idea per directory. Read these first. |
| `migrated/` | planned | Real-world corpus migrated from [`go-task/examples`](https://github.com/go-task/examples) via `rite --migrate`. Reference material and a migration-tool regression fence. |

`recipes/` is hand-written for clarity; `migrated/` will be mechanically produced and will preserve MIT attribution per file.

## CI smoke

The top-level `Ritefile.yml` runs `rite -l` against every Ritefile here — parse only, never execute. See `rite examples:verify`.
