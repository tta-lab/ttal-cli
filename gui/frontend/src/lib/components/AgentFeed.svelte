<script lang="ts">
	import Markdown from 'svelte-exmarkdown';
	import { gfmPlugin } from 'svelte-exmarkdown/gfm';
	import rehypeSanitize from 'rehype-sanitize';
	import rehypeHighlight from 'rehype-highlight';
	import 'highlight.js/styles/github-dark.css';
	import type { Message } from '../../../bindings/github.com/tta-lab/ttal-cli/internal/ent/models.js';
	import { formatTime } from '$lib/utils/time.js';

	interface Props {
		messages: Message[];
	}

	let { messages }: Props = $props();

	// rehype-sanitize runs before rehype-highlight to strip unsafe HTML
	const plugins = [
		gfmPlugin(),
		{ rehypePlugin: [rehypeSanitize] },
		{ rehypePlugin: [rehypeHighlight] }
	];

	// Feed shows oldest first (newest at bottom), reverse the DESC order from backend
	let chronological = $derived([...messages].reverse());
</script>

<div class="flex flex-col gap-2.5 p-3 px-4 overflow-y-auto flex-1">
	{#if chronological.length === 0}
		<div class="text-center text-neutral-content text-sm py-10 italic">No agent messages yet</div>
	{/if}
	{#each chronological as msg, i (msg.id?.toString() ?? `idx-${i}`)}
		<div class="card card-compact bg-base-300 border border-neutral">
			<div class="card-body">
				<div class="flex items-center gap-1.5 text-xs mb-1.5">
					<span class="font-semibold text-info">{msg.sender}</span>
					<span class="text-neutral-content">→</span>
					<span class="font-semibold text-success">{msg.recipient}</span>
					<span class="ml-auto text-neutral-content">{formatTime(msg.created_at)}</span>
				</div>
				<div class="prose prose-sm prose-invert max-w-none">
					<Markdown md={msg.content ?? ''} {plugins} />
				</div>
			</div>
		</div>
	{/each}
</div>
