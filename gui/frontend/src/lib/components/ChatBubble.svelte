<script lang="ts">
	import Markdown from 'svelte-exmarkdown';
	import { gfmPlugin } from 'svelte-exmarkdown/gfm';
	import rehypeSanitize from 'rehype-sanitize';
	import rehypeHighlight from 'rehype-highlight';
	import 'highlight.js/styles/github-dark.css';
	import type { Message } from '../../../bindings/github.com/tta-lab/ttal-cli/internal/ent/models.js';
	import { formatTime } from '$lib/utils/time.js';

	interface Props {
		message: Message;
		userName: string;
		avatarUrl?: string;
	}

	let { message, userName, avatarUrl }: Props = $props();

	// rehype-sanitize runs before rehype-highlight to strip unsafe HTML
	const plugins = [
		gfmPlugin(),
		{ rehypePlugin: [rehypeSanitize] },
		{ rehypePlugin: [rehypeHighlight] }
	];

	let isMine = $derived(message.sender === userName);
	let initials = $derived((message.sender ?? '?').slice(0, 1).toUpperCase());

</script>

<div class="chat" class:chat-start={!isMine} class:chat-end={isMine}>
	{#if !isMine}
		<div class="chat-image avatar">
			<div class="w-8 rounded-full bg-info/20">
				{#if avatarUrl}
					<img src={avatarUrl} alt={message.sender} />
				{:else}
					<span class="text-info text-sm font-semibold flex items-center justify-center w-full h-full">{initials}</span>
				{/if}
			</div>
		</div>
	{/if}
	{#if !isMine}
		<div class="chat-header text-neutral-content text-xs font-semibold">
			{message.sender}
		</div>
	{/if}
	<div class="chat-bubble" class:chat-bubble-accent={isMine}>
		<div class="prose prose-sm prose-invert max-w-none">
			<Markdown md={message.content ?? ''} {plugins} />
		</div>
	</div>
	<div class="chat-footer text-neutral-content/60">
		<time class="text-xs">{formatTime(message.created_at)}</time>
	</div>
</div>
