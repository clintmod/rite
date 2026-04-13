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
      { text: 'Migration', link: '/migration' },
      { text: 'Precedence', link: '/precedence' },
      { text: 'SPEC', link: 'https://github.com/clintmod/rite/blob/main/SPEC.md' },
    ],

    sidebar: [
      {
        text: 'Start here',
        items: [
          { text: 'What is rite?', link: '/' },
          { text: 'Getting started', link: '/getting-started' },
        ],
      },
      {
        text: 'Reference',
        items: [
          { text: 'Variable precedence', link: '/precedence' },
          { text: 'Syntax', link: '/syntax' },
          { text: 'CLI', link: '/cli' },
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
