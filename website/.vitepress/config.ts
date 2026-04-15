import { defineConfig } from 'vitepress'

// GitHub Pages on https://clintmod.github.io/rite/ serves from a sub-path;
// VitePress needs `base` to match so asset and internal links resolve.
// When a custom domain is configured later, `base` goes back to `/`.
const base = process.env.DOCS_BASE ?? '/rite/'

export default defineConfig({
  base,
  title: 'rite',
  description: 'An idempotent task runner with Unix-native variable precedence.',
  lang: 'en-US',
  srcDir: 'src',
  cleanUrls: true,
  lastUpdated: true,

  head: [
    ['meta', { name: 'keywords', content: 'task runner, ritefile, taskfile, build tool, make alternative' }],
  ],

  themeConfig: {
    nav: [
      { text: 'Docs', link: '/getting-started' },
      { text: 'Schema', link: '/schema' },
      { text: 'Migration', link: '/migration' },
      { text: 'SPEC', link: 'https://github.com/clintmod/rite/blob/main/SPEC.md' },
    ],

    sidebar: [
      {
        text: 'Start here',
        items: [
          { text: 'What is rite?', link: '/' },
          { text: 'Getting started', link: '/getting-started' },
          { text: 'Examples', link: '/examples' },
        ],
      },
      {
        text: 'Tasks',
        items: [
          { text: 'Short syntax', link: '/short-syntax' },
          { text: 'Internal tasks', link: '/internal-tasks' },
          { text: 'Aliases', link: '/aliases' },
          { text: 'Label override', link: '/label' },
          { text: 'Working directory', link: '/dir' },
          { text: 'Wildcard names', link: '/wildcards' },
          { text: 'Platforms', link: '/platforms' },
          { text: 'Calling another task', link: '/calling-tasks' },
          { text: 'Dependencies', link: '/deps' },
          { text: 'For-loops & matrices', link: '/for-loops' },
        ],
      },
      {
        text: 'Execution',
        items: [
          { text: 'Includes', link: '/includes' },
          { text: 'Run modes', link: '/run-modes' },
          { text: 'Incremental builds', link: '/sources-and-generates' },
          { text: 'Conditional execution (if)', link: '/conditionals' },
          { text: 'Preconditions & requires', link: '/preconditions' },
          { text: 'Defer (cleanup)', link: '/defer' },
          { text: 'Prompts', link: '/prompt' },
          { text: 'Silent / dry-run / ignore-error', link: '/silent-dry-ignore' },
          { text: 'Output timestamps', link: '/output-timestamps' },
          { text: 'Shell options (set/shopt)', link: '/set-shopt' },
          { text: 'Watch mode', link: '/watch' },
          { text: 'Interactive cmds', link: '/interactive' },
        ],
      },
      {
        text: 'Reference',
        items: [
          { text: 'Variable precedence', link: '/precedence' },
          { text: 'Syntax', link: '/syntax' },
          { text: 'CLI', link: '/cli' },
          { text: 'Validate', link: '/validate' },
          { text: 'Forwarding CLI args', link: '/cli-args' },
          { text: 'Special variables', link: '/special-vars' },
          { text: 'File discovery', link: '/file-discovery' },
          { text: 'CI integration', link: '/ci' },
          { text: 'Schema', link: '/schema' },
          { text: '.riterc config', link: '/riterc' },
        ],
      },
      {
        text: 'Coming from go-task',
        items: [{ text: 'Migration guide', link: '/migration' }],
      },
    ],

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
