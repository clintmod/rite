---
layout: home

hero:
  name: rite
  text: Task runner with Unix-native variable precedence.
  tagline: Your shell env wins. Your CLI args win. Internal `vars:` are defaults, not mandates. A hard fork of go-task where first-in-wins is actually first-in-wins.
  actions:
    - theme: brand
      text: Get started
      link: /next/getting-started
    - theme: alt
      text: Coming from go-task?
      link: /next/migration
    - theme: alt
      text: View on GitHub
      link: https://github.com/clintmod/rite

features:
  - title: First-in-wins precedence
    details: "Shell env beats CLI args beats entrypoint defaults beats task-scope. Task-scope vars blocks are defaults only — if any higher tier set the name, the task value is ignored. Seven documented tiers, no surprises."
    link: /next/precedence
    linkText: Read the precedence model
  - title: "${VAR} and {{.VAR}} are interchangeable"
    details: "Both syntaxes resolve against the same variable set with identical precedence. Use shell-native for commands, Go-template for conditionals and globs. Pick whichever reads cleaner."
    link: /next/syntax
    linkText: Read the syntax reference
  - title: Every var exports
    details: "A declared variable reaches the cmd shell environ by default. Opt out with export: false. No more vars vs env asymmetry."
    link: /next/precedence#env-blocks
    linkText: Read the env rules
  - title: One-way migration
    details: "rite --migrate Taskfile.yml produces a Ritefile and flags every site where first-in-wins would change the meaning of your existing config. No compatibility shim — an intentional break, documented line by line."
    link: /next/migration
    linkText: Read the migration guide
---

## Install

```sh
brew install clintmod/tap/rite                  # Homebrew
```
```toml
# mise.toml
[tools]
"ubi:clintmod/rite" = "v1.0.3"              # mise + ubi
```
```sh
go install github.com/clintmod/rite/cmd/rite@latest   # from source
```

More options (binary archives, deb/rpm/apk, older-mise fallbacks) on [the getting-started page](./getting-started#install).

## Why this exists

`go-task`'s variable model is structurally inverted: task-level `vars:` override CLI arguments and shell environment, which is the opposite of every Unix convention. The upstream project's planned redesign ([go-task/task#2035](https://github.com/go-task/task/issues/2035)) preserves the inversion.

`rite` starts from the opposite premise. The closer a variable is declared to the user, the more authority it has. Your shell environment is law. Internal `vars:` blocks declare what a value *should be if nothing else sets it*. Nothing a Ritefile declares internally can override what the user passed on the command line.

See [`SPEC.md`](https://github.com/clintmod/rite/blob/main/SPEC.md) for the full design contract.

## Why the name?

A **rite** is a ritual — a prescribed set of actions performed the same way every time. That's what a task runner is: a script of steps you repeat on every build, every deploy, every release. The word also reads as a near-homophone of *right*, which fits the project's thesis — variables should behave the way Unix has always done it, i.e. the *right* way. Short, typable, a nod to `task`'s spiritual ancestor `rake`, and doesn't collide with anything on `PATH`.

## The 60-second mental model

```yaml
# Ritefile.yml
version: '3'
vars:
  ENV: staging
tasks:
  deploy:
    vars:
      ENV: development   # task-scope default
    cmds:
      - echo "Deploying to ${ENV}"
```

| Invocation | Output | Why |
|---|---|---|
| `rite deploy` | `Deploying to staging` | Entrypoint `vars:` wins over task-scope default |
| `ENV=prod rite deploy` | `Deploying to prod` | Shell env wins over everything |
| `rite deploy ENV=qa` | `Deploying to qa` | CLI wins over entrypoint and task-scope |

Compare to `go-task`, where `rite deploy ENV=qa` would print `development` because task-scope `vars:` override CLI args.
