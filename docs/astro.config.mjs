// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
	site: 'https://ttal.guion.io',
	integrations: [
		starlight({
			title: 'ttal',
			customCss: ['./src/styles/custom.css'],
			description: 'Manage your coding agents from your phone. Multi-agent orchestration for Claude Code, OpenCode, and Codex CLI.',
			social: [
				{ icon: 'codeberg', label: 'Codeberg', href: 'https://codeberg.org/clawteam/ttal-cli' },
			],
			sidebar: [
				{ label: 'About', slug: 'about' },
				{ label: 'Pricing', slug: 'pricing' },
				{
					label: 'Documentation',
					autogenerate: { directory: 'docs' },
				},
				{
					label: 'Guides',
					autogenerate: { directory: 'guides' },
				},
				{
					label: 'Blog',
					autogenerate: { directory: 'blog' },
				},
			],
			head: [
				{
					tag: 'meta',
					attrs: {
						property: 'og:type',
						content: 'website',
					},
				},
			],
		}),
	],
});
