# Output timestamps

Prefix every line rite emits — cmd stdout/stderr **and** rite's own `rite:` log lines — with a timestamp. The motivation is scrollback archaeology: when a long task takes too long, you should be able to read straight off the log where the time went, without first splitting rite's lines from the cmd's.

go-task has never shipped this ([go-task/task#1065](https://github.com/go-task/task/issues/1065)). `rite` owns its entire output pipeline so the prefix lands uniformly on every byte that leaves the process.

## Three ways to turn it on

Precedence, highest-wins first: **CLI / env > task > top-level**.

### 1. CLI flag or env var

Highest precedence. Useful when debugging a single run without touching the Ritefile.

```sh
rite --timestamps build                   # default ISO-8601 UTC ms layout
rite --timestamps="[%H:%M:%S]" build      # custom strftime format
RITE_TIMESTAMPS=1 rite build              # env var, same grammar
RITE_TIMESTAMPS=false rite build          # explicit override OFF (escape hatch)
```

### 2. Top-level (project-wide default)

Sibling of `set:` / `output:` at the entrypoint — opts every task in:

```yaml
version: '3'
timestamps: true
tasks:
  build:
    cmds:
      - go build ./...
```

### 3. Task-level (per-task override)

Turn one task off (common) or give it a different format:

```yaml
version: '3'
timestamps: true
tasks:
  build:
    cmds:
      - go build ./...
  repl:
    timestamps: false        # opt this interactive task out
    cmds:
      - node --interactive
```

## The default format

Bare `true` (at any scope) renders a fixed-width bracketed ISO 8601 prefix:

```
[2026-04-15T14:23:01.123Z] go: downloading ...
```

- Always UTC. `rite` calls `.UTC()` on the underlying `time.Time` so the host `$TZ` never leaks in.
- Milliseconds are zero-padded to exactly three digits (`.000`, not `.9`) — column-oriented log tools assume fixed width.
- Square brackets keep the prefix visually separable from the cmd content.

The default layout is anchored in code as `ast.DefaultTimestampLayout` so tooling and docs agree on the exact shape.

## Custom strftime formats

Pass any string containing one or more of the supported strftime tokens:

| token | renders as | notes |
| ----- | --------- | ----- |
| `%Y` | `2026` | four-digit year |
| `%m` | `04` | zero-padded month |
| `%d` | `15` | zero-padded day of month |
| `%H` | `14` | zero-padded hour (24h) |
| `%M` | `23` | zero-padded minute |
| `%S` | `01` | zero-padded second |
| `%L` | `.123` | three-digit millisecond, dot-prefixed (matches `ts -s %.S`) |
| `%z` | `-0700` | timezone offset |
| `%%` | `%` | literal percent |

Anything else between `%` markers passes through as a literal. An unsupported token (`%Q`, etc.) is a hard error at Ritefile parse time — rite does not silently ignore unknown strftime codes.

Examples:

```yaml
timestamps: "[%H:%M:%S]"           # [14:23:01]
timestamps: "%Y-%m-%dT%H:%M:%S%L"  # 2026-04-15T14:23:01.123
```

Custom formats render in local time unless they include `%z` or a literal UTC marker. The `true` default is the only UTC-forced form.

## Interactive tasks opt out

When a task needs an interactive TTY (spinners, REPLs, Xcode's `-showBuildSettings` cursor dance), timestamp prefixes fight the cmd's redraws. The pattern is project-wide-on plus task-level-off:

```yaml
version: '3'
timestamps: true      # every task logs with timestamps...
tasks:
  deploy:
    cmds:
      - kubectl apply -f manifests/
  debug-pod:
    timestamps: false # ...except this one, which wants a raw TTY
    cmds:
      - kubectl exec -it {{.POD}} -- /bin/bash
```

A CLI `--timestamps` preempts the task's `false` — if you explicitly ask for timestamps on the command line, you get them, even on a task that tried to opt out. Pragmatic: debugging an interactive flow sometimes *needs* timestamps even at the cost of visual noise.

::: tip Scope note
Task-level `timestamps:` controls the task's **cmd output** (the bytes the shell writes). rite's own `rite: [task] cmd` preamble line — which prints *before* the cmd runs — uses the **global** (CLI / top-level) setting. The preamble happens before any interactive TTY handoff, so stamping it is harmless and keeps rite's log voice consistent across tasks.
:::

## Interaction with other features

- **`silent:` tasks** emit nothing to prefix, so timestamps are a no-op there.
- **`output: group`** banners (`begin:` / `end:`) are rite's own output inside the output pipeline and are themselves timestamped.
- **`output: prefixed`** labels come *after* the timestamp: `[2026-04-15T14:23:01.123Z] [task:foo] some line`.
- **ANSI color escapes** are preserved — the timestamp lands before the first byte of the line, so color sequences aren't split.
- **Partial lines** (a `printf '%s'` without a trailing newline) are buffered until newline arrives or the cmd exits. On exit rite flushes them with a final timestamp and a synthesized `\n` so log consumers never see half-stamped output.

## Nested rite invocations

When a `cmds:` entry shells out to another `rite` invocation, only the outermost rite stamps. A 3-deep chain (`default` → `rite middle` → `rite leaf` → `printf hello`) under `RITE_TIMESTAMPS=1` emits exactly one prefix per line, not three.

The mechanism: whenever rite wraps a cmd's output with a timestamp writer, it also sets `RITE_TIMESTAMPS_HANDLED=1` in that cmd's environ. A nested rite that sees this marker in its own environ suppresses its timestamp wrapping — cmd output and logger lines alike — so the outer rite is the single source of prefixes. Non-rite children (`npm`, `bundle`, `cargo`) see the variable and ignore it.

An inner `--timestamps` flag is deliberately *also* suppressed by the marker: re-wrapping already-prefixed output is the bug this prevents, not a feature. If you genuinely want nested stamping on a specific sub-invocation (unusual), clear the marker on that cmd:

```yaml
tasks:
  default:
    cmds:
      - cmd: rite inner
        env:
          RITE_TIMESTAMPS_HANDLED: ''   # opt this one child back into wrapping
```

In-process subcalls via `cmds: - task: foo` never fork a process and were never affected by the bug; they continue to produce one prefix per line through the existing single-wrap path.

## Env var grammar

`RITE_TIMESTAMPS` accepts the same values as the flag, plus common variants:

| value | meaning |
| ----- | ------- |
| `1`, `true`, `on` | on, default layout |
| `0`, `false`, `off` | off (escape hatch) |
| `<strftime>` | on, custom format |
| unset or empty | no CLI-level override (falls through to Ritefile) |
