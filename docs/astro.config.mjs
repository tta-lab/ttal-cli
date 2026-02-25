// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
	site: 'https://ttal.guion.io',
	integrations: [
		starlight({
			title: 'The Taskwarrior Agents Lab',
			description: 'AIOps workflows with Taskwarrior, Zellij, and Claude Code',
			social: [
				{ icon: 'github', label: 'GitHub', href: 'https://github.com/neilguion' },
			],
			sidebar: [
				{ label: 'About', slug: 'about' },
				{
					label: 'Guides',
					autogenerate: { directory: 'guides' },
				},
				{
					label: 'Projects',
					items: [
						{ label: 'AIOps System', slug: 'projects/aiops' },
					],
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
