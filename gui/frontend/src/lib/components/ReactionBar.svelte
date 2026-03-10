<script lang="ts">
	import type { Reaction } from '../../../bindings/github.com/tta-lab/ttal-cli/internal/ent/models.js';
	import * as ChatService from '../../../bindings/github.com/tta-lab/ttal-cli/gui/chatservice.js';
	import 'emoji-picker-element';
	import type { EmojiClickEvent } from 'emoji-picker-element/shared';

	interface Props {
		messageId: string;
		reactions: Reaction[];
	}

	let { messageId, reactions }: Props = $props();

	let showPicker = $state(false);
	let pickerEl = $state<HTMLElement | null>(null);
	let containerEl = $state<HTMLElement | null>(null);

	// Group reactions by emoji for badge display
	let grouped = $derived(
		Object.entries(
			reactions.reduce<Record<string, number>>((acc, r) => {
				const key = r.emoji ?? '';
				acc[key] = (acc[key] ?? 0) + 1;
				return acc;
			}, {})
		)
	);

	// Attach emoji-click listener when picker mounts
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

	// Close picker on click outside the container
	$effect(() => {
		if (!showPicker) return;
		const handler = (e: MouseEvent) => {
			if (containerEl && !containerEl.contains(e.target as Node)) {
				showPicker = false;
			}
		};
		document.addEventListener('mousedown', handler);
		return () => document.removeEventListener('mousedown', handler);
	});

	function togglePicker() {
		showPicker = !showPicker;
	}
</script>

<div class="flex flex-wrap gap-1 items-center mt-1 relative" bind:this={containerEl}>
	{#each grouped as [emoji, count] (emoji)}
		<button class="badge badge-sm badge-outline gap-1 cursor-pointer hover:bg-accent" title="React with {emoji}" onclick={togglePicker}>
			{emoji} <span class="text-xs text-base-content/60">{count}</span>
		</button>
	{/each}

	<button class="badge badge-sm badge-outline badge-ghost cursor-pointer hover:bg-secondary" title="Add reaction" onclick={togglePicker}>＋</button>

	{#if showPicker}
		<!-- svelte-ignore element_invalid_self_closing_tag -->
		<div class="absolute bottom-full left-0 z-50 mb-1">
			<emoji-picker bind:this={pickerEl}></emoji-picker>
		</div>
	{/if}
</div>
