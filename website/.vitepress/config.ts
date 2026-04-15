import { defineConfig } from 'vitepress'
import { readdirSync, existsSync } from 'node:fs'
import { resolve } from 'node:path'

// GitHub Pages on https://clintmod.github.io/rite/ serves from a sub-path;
// VitePress needs `base` to match so asset and internal links resolve.
// When a custom domain is configured later, `base` goes back to `/`.
const base = process.env.DOCS_BASE ?? '/rite/'

// Discover versioned dirs from the filesystem at build time.
// Layout: website/src/next/ (main-branch docs) + website/src/vX.Y.Z/ (snapshots).
// See docs/issue-132 for the versioning scheme.
const SRC_DIR = resolve(__dirname, '../src')

function semverDesc(a: string, b: string): number {
  // Sort vX.Y.Z descending. "next" handled separately.
  const pa = a.slice(1).split('.').map(n => parseInt(n, 10))
  const pb = b.slice(1).split('.').map(n => parseInt(n, 10))
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const da = pa[i] ?? 0
    const db = pb[i] ?? 0
    if (da !== db) return db - da
  }
  return 0
}

function discoverVersions(): { versions: string[]; latest: string } {
  const entries = readdirSync(SRC_DIR, { withFileTypes: true })
    .filter(d => d.isDirectory())
    .map(d => d.name)
  const tagged = entries.filter(n => /^v\d+\.\d+\.\d+$/.test(n)).sort(semverDesc)
  const hasNext = entries.includes('next')
  const versions = [...(hasNext ? ['next'] : []), ...tagged]
  const latest = tagged[0] ?? 'next'
  return { versions, latest }
}

const { versions, latest } = discoverVersions()

// Sidebar shape is shared across versions — same chapter layout, but links
// prefixed with the version dir so internal navigation stays within-version.
function sidebarFor(version: string) {
  const p = (slug: string) => `/${version}/${slug}`
  return [
    {
      text: 'Start here',
      items: [
        { text: 'What is rite?', link: `/${version}/` },
        { text: 'Getting started', link: p('getting-started') },
        { text: 'Examples', link: p('examples') },
      ],
    },
    {
      text: 'Tasks',
      items: [
        { text: 'Short syntax', link: p('short-syntax') },
        { text: 'Internal tasks', link: p('internal-tasks') },
        { text: 'Aliases', link: p('aliases') },
        { text: 'Label override', link: p('label') },
        { text: 'Working directory', link: p('dir') },
        { text: 'Wildcard names', link: p('wildcards') },
        { text: 'Platforms', link: p('platforms') },
        { text: 'Calling another task', link: p('calling-tasks') },
        { text: 'Dependencies', link: p('deps') },
        { text: 'For-loops & matrices', link: p('for-loops') },
      ],
    },
    {
      text: 'Execution',
      items: [
        { text: 'Includes', link: p('includes') },
        { text: 'Run modes', link: p('run-modes') },
        { text: 'Incremental builds', link: p('sources-and-generates') },
        { text: 'Conditional execution (if)', link: p('conditionals') },
        { text: 'Preconditions & requires', link: p('preconditions') },
        { text: 'Defer (cleanup)', link: p('defer') },
        { text: 'Prompts', link: p('prompt') },
        { text: 'Silent / dry-run / ignore-error', link: p('silent-dry-ignore') },
        { text: 'Output timestamps', link: p('output-timestamps') },
        { text: 'Shell options (set/shopt)', link: p('set-shopt') },
        { text: 'Watch mode', link: p('watch') },
        { text: 'Interactive cmds', link: p('interactive') },
      ],
    },
    {
      text: 'Reference',
      items: [
        { text: 'Variable precedence', link: p('precedence') },
        { text: 'Syntax', link: p('syntax') },
        { text: 'CLI', link: p('cli') },
        { text: 'Validate', link: p('validate') },
        { text: 'Forwarding CLI args', link: p('cli-args') },
        { text: 'Special variables', link: p('special-vars') },
        { text: 'File discovery', link: p('file-discovery') },
        { text: 'CI integration', link: p('ci') },
        { text: 'Schema', link: p('schema') },
        { text: '.riterc config', link: p('riterc') },
      ],
    },
    {
      text: 'Coming from go-task',
      items: [{ text: 'Migration guide', link: p('migration') }],
    },
  ]
}

// Build a sidebar map keyed by version path prefix. Only include pages
// that actually exist for that version — older snapshots predate some chapters.
const sidebar: Record<string, ReturnType<typeof sidebarFor>> = {}
for (const v of versions) {
  const full = sidebarFor(v)
  // Prune entries whose underlying .md doesn't exist in this version's dir.
  // index is special — its slug is '' (empty, resolves to dir root).
  const versionDir = resolve(SRC_DIR, v)
  const pruned = full
    .map(group => ({
      ...group,
      items: group.items.filter(item => {
        const link = item.link as string
        const slug = link.replace(`/${v}/`, '').replace(`/${v}`, '').split('#')[0]
        if (slug === '' || slug === '/') return existsSync(resolve(versionDir, 'index.md'))
        return existsSync(resolve(versionDir, `${slug}.md`))
      }),
    }))
    .filter(group => group.items.length > 0)
  sidebar[`/${v}/`] = pruned
}

// Version switcher for the nav bar. Renders as a dropdown with the
// currently-active version at the top label.
const versionNav = {
  text: `Version: ${latest === 'next' ? 'next' : latest}`,
  items: versions.map(v => ({
    text: v === 'next' ? 'next (unreleased)' : v,
    link: `/${v}/`,
  })),
}

export default defineConfig({
  base,
  title: 'rite',
  description: 'An idempotent task runner with Unix-native variable precedence.',
  lang: 'en-US',
  srcDir: 'src',
  cleanUrls: true,
  lastUpdated: true,
  ignoreDeadLinks: true,

  // Make '/' resolve to the latest released version's homepage.
  // The physical file website/src/index.md is a stub that redirects via
  // VitePress rewrites (see below); this keeps bookmarks/short-URL targets stable.
  rewrites: {
    'index.md': 'index.md',
  },

  head: [
    ['meta', { name: 'keywords', content: 'task runner, ritefile, taskfile, build tool, make alternative' }],
    // Client-side redirect from / to the latest version.
    ['script', {}, `
      (function() {
        if (typeof window === 'undefined') return;
        var path = window.location.pathname;
        var base = '${base}';
        if (path === base || path === base.replace(/\\/$/, '') || path === '/') {
          window.location.replace(base + '${latest}/');
        }
      })();
    `],
  ],

  themeConfig: {
    nav: [
      { text: 'Docs', link: `/${latest}/getting-started` },
      { text: 'Schema', link: `/${latest}/schema` },
      { text: 'Migration', link: `/${latest}/migration` },
      versionNav,
      { text: 'SPEC', link: 'https://github.com/clintmod/rite/blob/main/SPEC.md' },
    ],

    sidebar,

    socialLinks: [
      { icon: 'github', link: 'https://github.com/clintmod/rite' },
    ],

    editLink: {
      pattern: 'https://github.com/clintmod/rite/edit/main/website/src/:path',
      text: 'Edit this page on GitHub',
    },

    footer: {
      message: 'MIT licensed · hard fork of go-task',
      copyright: 'Original © 2016 Andrey Nering · Fork © 2026 Clint Modien',
    },

    search: { provider: 'local' },
  },
})
