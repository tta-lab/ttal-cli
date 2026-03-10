<script lang="ts">
	import Markdown from 'svelte-exmarkdown';
	import { gfmPlugin } from 'svelte-exmarkdown/gfm';
	import rehypeHighlight from 'rehype-highlight';
	import 'highlight.js/styles/github-dark.css';
	import type { Message } from '../../../bindings/github.com/tta-lab/ttal-cli/internal/ent/models.js';

	interface Props {
		messages: (Message | null)[];
	}

	let { messages }: Props = $props();

	const plugins = [gfmPlugin(), { rehypePlugin: [rehypeHighlight] }];

	// Feed shows oldest first (newest at bottom), reverse the DESC order from backend
	let chronological = $derived([...messages].reverse().filter(Boolean) as Message[]);

	function formatTime(ts: unknown): string {
		if (!ts) return '';
		const d = new Date(ts as string);
		if (isNaN(d.getTime())) return '';
		return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
	}
</script>

<div class="feed">
	{#if chronological.length === 0}
		<div class="empty">No agent messages yet</div>
	{/if}
	{#each chronological as msg (msg.id?.toString())}
		<div class="feed-entry">
			<div class="feed-meta">
				<span class="feed-sender">{msg.sender}</span>
				<span class="feed-arrow">→</span>
				<span class="feed-recipient">{msg.recipient}</span>
				<span class="feed-time">{formatTime(msg.created_at)}</span>
			</div>
			<div class="feed-body">
				<Markdown md={msg.content ?? ''} {plugins} />
			</div>
		</div>
	{/each}
</div>

<style>
	.feed {
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 12px 16px;
		overflow-y: auto;
		flex: 1;
	}

	.empty {
		text-align: center;
		color: #475569;
		font-size: 0.85rem;
		padding: 40px 0;
		font-style: italic;
	}

	.feed-entry {
		background: #1a2030;
		border: 1px solid #2a3348;
		border-radius: 8px;
		padding: 10px 12px;
	}

	.feed-meta {
		display: flex;
		align-items: center;
		gap: 6px;
		margin-bottom: 6px;
		font-size: 0.75rem;
	}

	.feed-sender {
		font-weight: 600;
		color: #93c5fd;
	}

	.feed-arrow {
		color: #475569;
	}

	.feed-recipient {
		font-weight: 600;
		color: #86efac;
	}

	.feed-time {
		margin-left: auto;
		color: #475569;
	}

	.feed-body {
		font-size: 0.82rem;
		color: #cbd5e1;
		line-height: 1.5;
	}

	.feed-body :global(p) {
		margin: 0 0 4px;
	}

	.feed-body :global(p:last-child) {
		margin-bottom: 0;
	}

	.feed-body :global(pre) {
		overflow-x: auto;
		border-radius: 6px;
		padding: 8px 10px;
		background: #0f172a;
		font-size: 0.78rem;
		margin: 4px 0;
	}

	.feed-body :global(code:not(pre code)) {
		background: #0f172a;
		border-radius: 3px;
		padding: 1px 4px;
		font-size: 0.8em;
	}
</style>
