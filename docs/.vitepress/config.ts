import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'TTAL',
  description: 'Manage your coding agents from your phone. Multi-agent orchestration for Claude Code and Codex CLI.',

  head: [
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:site_name', content: 'ttal' }],
  ],

  // Output to ./dist for wrangler compatibility
  outDir: './dist',

  // Exclude internal docs from build
  srcExclude: ['plans/**', 'posts/**', 'AIOPS.md', 'DATABASE.md', 'ECOSYSTEM.md', 'STT_SETUP.md', 'TELEGRAM_LIB_DECISION.md', 'VOICE_SETUP.md'],

  themeConfig: {
    nav: [
      { text: 'Docs', link: '/docs/getting-started' },
      { text: 'Guides', link: '/guides/research-design-execute' },
      { text: 'Blog', link: '/blog/the-dream-setup' },
      { text: 'Pricing', link: '/pricing' },
      { text: 'Roadmap', link: '/roadmap' },
    ],

    sidebar: {
      '/docs/': [
        {
          text: 'Documentation',
          items: [
            { text: 'Getting Started', link: '/docs/getting-started' },
            { text: 'Configuration', link: '/docs/configuration' },
            { text: 'Agents', link: '/docs/agents' },
            { text: 'Messaging', link: '/docs/messaging' },
            { text: 'Workers', link: '/docs/workers' },
            { text: 'Tasks', link: '/docs/tasks' },
            { text: 'Prompts', link: '/docs/prompts' },
            { text: 'Daemon', link: '/docs/daemon' },
            { text: 'Runtimes', link: '/docs/runtimes' },
            { text: 'Voice', link: '/docs/voice' },
          ]
        }
      ],
      '/guides/': [
        {
          text: 'Guides',
          items: [
            { text: 'Research → Design → Execute', link: '/guides/research-design-execute' },
            { text: 'Custom Tag Routing', link: '/guides/custom-tag-routing' },
            { text: 'Building Your Team', link: '/guides/building-your-team' },
            { text: 'PR Review Workflow', link: '/guides/pr-review-workflow' },
          ]
        }
      ],
      '/blog/': [
        {
          text: 'Blog',
          items: [
            { text: 'The Dream Setup', link: '/blog/the-dream-setup' },
            { text: 'The Glue Layer', link: '/blog/the-glue-layer' },
            { text: 'Philosophy', link: '/blog/philosophy' },
          ]
        }
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/tta-lab/ttal-cli' },
    ],

    footer: {
      message: 'MIT License',
      copyright: '© 2025-present GuionAI',
    },

    search: {
      provider: 'local',
    },
  },
})
