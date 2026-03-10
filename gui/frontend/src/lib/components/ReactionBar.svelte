<script lang="ts">
	import { onMount } from 'svelte';
	import type { Reaction } from '../../../bindings/github.com/tta-lab/ttal-cli/internal/ent/models.js';
	import * as ChatService from '../../../bindings/github.com/tta-lab/ttal-cli/gui/chatservice.js';
	import 'emoji-picker-element';

	interface Props {
		messageId: string;
		reactions: (Reaction | null)[];
	}

	let { messageId, reactions }: Props = $props();

	let showPicker = $state(false);
	let pickerEl = $state<HTMLElement | null>(null);

	// Group reactions by emoji for badge display
	let grouped = $derived(
		Object.entries(
			reactions
				.filter(Boolean)
				.reduce<Record<string, number>>((acc, r) => {
					const key = r!.emoji ?? '';
					acc[key] = (acc[key] ?? 0) + 1;
					return acc;
				}, {})
		)
	);

	function togglePicker() {
		showPicker = !showPicker;
	}

	async function handleEmojiClick(e: Event) {
		const detail = (e as CustomEvent).detail?.unicode as string | undefined;
		if (!detail) return;
		showPicker = false;
		try {
			await ChatService.AddReaction(messageId, detail);
		} catch (err) {
			console.error('AddReaction failed:', err);
		}
	}

	onMount(() => {
		if (pickerEl) {
			pickerEl.addEventListener('emoji-click', handleEmojiClick);
			return () => pickerEl?.removeEventListener('emoji-click', handleEmojiClick);
		}
	});
</script>

<div class="reaction-bar">
	{#each grouped as [emoji, count] (emoji)}
		<button class="reaction-badge" title="React with {emoji}" onclick={togglePicker}>
			{emoji} <span class="count">{count}</span>
		</button>
	{/each}

	<button class="add-reaction" title="Add reaction" onclick={togglePicker}>＋</button>

	{#if showPicker}
		<!-- svelte-ignore element_invalid_self_closing_tag -->
		<div class="picker-wrapper">
			<emoji-picker bind:this={pickerEl}></emoji-picker>
		</div>
	{/if}
</div>

<style>
	.reaction-bar {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
		align-items: center;
		margin-top: 4px;
		position: relative;
	}

	.reaction-badge {
		display: inline-flex;
		align-items: center;
		gap: 3px;
		padding: 2px 6px;
		border-radius: 12px;
		background: #1e2a40;
		border: 1px solid #2a3348;
		font-size: 0.8rem;
		cursor: pointer;
		color: #e2e8f0;
		transition: background 0.1s;
	}

	.reaction-badge:hover {
		background: #1e3a5f;
	}

	.count {
		font-size: 0.7rem;
		color: #94a3b8;
	}

	.add-reaction {
		padding: 1px 6px;
		border-radius: 12px;
		background: transparent;
		border: 1px solid #2a3348;
		font-size: 0.75rem;
		cursor: pointer;
		color: #64748b;
		width: auto;
		height: auto;
		line-height: normal;
		transition: all 0.1s;
	}

	.add-reaction:hover {
		background: #1e2a40;
		color: #e2e8f0;
	}

	.picker-wrapper {
		position: absolute;
		bottom: calc(100% + 4px);
		left: 0;
		z-index: 100;
	}

	:global(emoji-picker) {
		--background: #1a2030;
		--border-color: #2a3348;
		--text-color: #e2e8f0;
		--input-background: #0f172a;
		--input-border-radius: 6px;
		--num-columns: 8;
		width: 320px;
		height: 280px;
	}
</style>
