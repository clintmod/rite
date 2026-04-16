---
title: Docs versioning
description: How rite's docs site mirrors each released version — and how to pin a specific one.
---

# Docs versioning

The docs site hosts a separate tree for every release plus a bleeding-edge
`next/` view of `main`. Pick the one that matches your installed binary
with the **Version** dropdown in the top nav — or URL-pin it directly:

| URL prefix | What you get |
|------------|--------------|
| `/`        | Redirects to the latest **released** version |
| `/next/`   | The bleeding-edge docs on `main`, may describe unreleased features |
| `/v1.0.3/` | Frozen docs as they shipped with the `v1.0.3` release |
| `/v1.0.2/` | …and so on for every past release |

Run `rite --version` to see what your binary reports, and visit the
matching `/v<VERSION>/` path.

## Why per-version docs?

The install examples in the docs embed concrete version pins
(`brew install rite`, `ubi:clintmod/rite@vX.Y.Z`). When those get bumped
on `main` but you're running an older binary, the docs start describing
flags and syntax you don't have yet. Per-version docs mean the page you
land on matches the code you're running.

## Fixing a typo in an old version

Old-version dirs are **not** frozen — they're just files in `main`. Find
the file under `website/src/v<VERSION>/`, open a PR, ship. Next merge to
`main` re-deploys that version's path with the fix. No re-release needed.

## Reporting docs bugs

Open an issue at
<https://github.com/clintmod/rite/issues/new> and paste the full URL
(including the `/v<VERSION>/` prefix) so we know which version's docs
you're reading.

## How it's wired

- Each release, `rite release:prepare VERSION=X.Y.Z` runs
  `release:snapshot-docs` which copies `website/src/next/` into
  `website/src/v<X.Y.Z>/`. The snapshot lands in the staging PR.
- VitePress discovers version dirs at build time by scanning
  `website/src/` for `vX.Y.Z/` subdirs; sidebars and the version dropdown
  are generated from that list.
- `SPEC.md` and `CHANGELOG.md` aren't inlined into versioned dirs (the
  site's Vue toolchain chokes on Go-template syntax in fenced blocks) —
  each version's Spec / Changelog pages are thin stubs that link out to
  the tag-pinned blobs on GitHub.
