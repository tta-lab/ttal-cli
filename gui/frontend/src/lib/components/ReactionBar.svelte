<script lang="ts">
	import type { Reaction } from '../../../bindings/github.com/tta-lab/ttal-cli/internal/ent/models.js';
	import * as ChatService from '../../../bindings/github.com/tta-lab/ttal-cli/gui/chatservice.js';
	import 'emoji-picker-element';
	import type { EmojiClickEvent } from 'emoji-picker-element/shared';

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

	// $effect re-runs whenever pickerEl changes (i.e. when {#if showPicker} renders it)
	$effect(() => {
		if (!pickerEl) return;
		const handler = async (e: EmojiClickEvent) => {
			const emoji = e.detail.unicode;
			if (!emoji) return;
			showPicker = false;
			try {
				await ChatService.AddReaction(messageId, emoji);
			} catch (err) {
				console.error('AddReaction failed:', err);
			}
		};
		pickerEl.addEventListener('emoji-click', handler as EventListener);
		return () => pickerEl?.removeEventListener('emoji-click', handler as EventListener);
	});

	function togglePicker() {
		showPicker = !showPicker;
	}
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
		width: auto;
		height: auto;
		line-height: normal;
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
