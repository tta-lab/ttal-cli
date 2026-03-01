import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'TTAL',
  description: 'Manage your coding agents from your phone. Multi-agent orchestration for Claude Code, OpenCode, and Codex CLI.',

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
      { text: 'Blog', link: '/blog/day-1-the-dream-setup' },
      { text: 'Pricing', link: '/pricing' },
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
            { text: 'Day 1: The Dream Setup', link: '/blog/day-1-the-dream-setup' },
            { text: 'Day 2: OpenClaw Overview', link: '/blog/day-2-openclaw-overview' },
            { text: 'Day 3: Zellij & Coding Agents', link: '/blog/day-3-zellij-coding-agents' },
            { text: 'Day 4: Taskwarrior Deep Dive', link: '/blog/day-4-taskwarrior-deep-dive' },
          ]
        }
      ],
    },

    socialLinks: [
      { icon: { svg: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="currentColor" d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8z"/></svg>' }, link: 'https://codeberg.org/clawteam/ttal-cli' },
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
