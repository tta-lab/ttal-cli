<script lang="ts">
	import Markdown from 'svelte-exmarkdown';
	import { gfmPlugin } from 'svelte-exmarkdown/gfm';
	import rehypeHighlight from 'rehype-highlight';
	import 'highlight.js/styles/github-dark.css';
	import type { Message } from '../../../bindings/github.com/tta-lab/ttal-cli/internal/ent/models.js';
	import ReactionBar from './ReactionBar.svelte';

	interface Props {
		message: Message;
		userName: string;
		avatarUrl?: string;
	}

	let { message, userName, avatarUrl }: Props = $props();

	const plugins = [gfmPlugin(), { rehypePlugin: [rehypeHighlight] }];

	let isMine = $derived(message.sender === userName);
	let initials = $derived((message.sender ?? '?').slice(0, 1).toUpperCase());

	function formatTime(ts: unknown): string {
		if (!ts) return '';
		const d = new Date(ts as string);
		if (isNaN(d.getTime())) return '';
		return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
	}

	let reactions = $derived(message.edges?.reactions ?? []);
	let messageId = $derived(message.id?.toString() ?? '');
</script>

<div class="bubble-row" class:mine={isMine}>
	{#if !isMine}
		<div class="avatar" title={message.sender}>
			{#if avatarUrl}
				<img src={avatarUrl} alt={message.sender} />
			{:else}
				{initials}
			{/if}
		</div>
	{/if}

	<div class="bubble-content">
		{#if !isMine}
			<span class="sender-name">{message.sender}</span>
		{/if}
		<div class="bubble" class:mine={isMine}>
			<div class="markdown-body">
				<Markdown md={message.content ?? ''} {plugins} />
			</div>
		</div>
		<div class="meta-row">
			<span class="timestamp">{formatTime(message.created_at)}</span>
		</div>
		{#if reactions.length > 0 && messageId}
			<ReactionBar {messageId} {reactions} />
		{:else if messageId}
			<ReactionBar {messageId} reactions={[]} />
		{/if}
	</div>
</div>

<style>
	.bubble-row {
		display: flex;
		align-items: flex-end;
		gap: 8px;
		margin-bottom: 12px;
	}

	.bubble-row.mine {
		flex-direction: row-reverse;
	}

	.avatar {
		width: 32px;
		height: 32px;
		border-radius: 50%;
		background: #2d4a7a;
		color: #93c5fd;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 0.85rem;
		font-weight: 600;
		flex-shrink: 0;
		overflow: hidden;
	}

	.avatar img {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	.bubble-content {
		display: flex;
		flex-direction: column;
		max-width: 72%;
	}

	.sender-name {
		font-size: 0.7rem;
		font-weight: 600;
		color: #64748b;
		margin-bottom: 3px;
		padding-left: 2px;
	}

	.bubble {
		background: #1e2a40;
		border-radius: 12px 12px 12px 4px;
		padding: 8px 12px;
		color: #e2e8f0;
		font-size: 0.875rem;
		line-height: 1.5;
		word-break: break-word;
	}

	.bubble.mine {
		background: #1e3a5f;
		border-radius: 12px 12px 4px 12px;
	}

	.markdown-body :global(p) {
		margin: 0 0 6px;
	}

	.markdown-body :global(p:last-child) {
		margin-bottom: 0;
	}

	.markdown-body :global(pre) {
		overflow-x: auto;
		border-radius: 6px;
		margin: 6px 0;
		padding: 10px 12px;
		font-size: 0.8rem;
		background: #0f172a;
	}

	.markdown-body :global(code:not(pre code)) {
		background: #0f172a;
		border-radius: 3px;
		padding: 1px 5px;
		font-size: 0.82em;
	}

	.markdown-body :global(table) {
		border-collapse: collapse;
		font-size: 0.82rem;
		margin: 6px 0;
	}

	.markdown-body :global(th),
	.markdown-body :global(td) {
		border: 1px solid #2a3348;
		padding: 4px 8px;
	}

	.markdown-body :global(blockquote) {
		border-left: 3px solid #2a3348;
		padding-left: 10px;
		margin: 4px 0;
		color: #94a3b8;
	}

	.meta-row {
		display: flex;
		align-items: center;
		gap: 6px;
		margin-top: 3px;
		padding: 0 2px;
	}

	.bubble-row.mine .meta-row {
		justify-content: flex-end;
	}

	.timestamp {
		font-size: 0.65rem;
		color: #475569;
	}
</style>
