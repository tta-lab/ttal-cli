<script lang="ts">
	import type { TeamInfo } from '../../../bindings/github.com/tta-lab/ttal-cli/gui/models.js';

	interface Props {
		teams: TeamInfo[];
		onSelect: (team: string, agent: string) => void;
		onClose: () => void;
	}

	let { teams, onSelect, onClose }: Props = $props();
	let query = $state('');
	let selectedIndex = $state(0);

	$effect(() => {
		// Reset selection whenever query changes
		query;
		selectedIndex = 0;
	});

	let filtered = $derived(
		teams
			.flatMap((t) => t.agents.map((a) => ({ team: t.name, ...a })))
			.filter((a) => a.name.toLowerCase().includes(query.toLowerCase()))
	);

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'ArrowDown') {
			selectedIndex = Math.min(selectedIndex + 1, filtered.length - 1);
			e.preventDefault();
		} else if (e.key === 'ArrowUp') {
			selectedIndex = Math.max(selectedIndex - 1, 0);
			e.preventDefault();
		} else if (e.key === 'Enter' && filtered[selectedIndex]) {
			const item = filtered[selectedIndex];
			onSelect(item.team, item.name);
		} else if (e.key === 'Escape') {
			onClose();
		}
	}
</script>

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div class="overlay" onclick={onClose}>
	<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
	<div class="palette" onclick={(e) => e.stopPropagation()}>
		<input
			class="palette-input"
			placeholder="Search agents…"
			bind:value={query}
			onkeydown={handleKeydown}
			autofocus
		/>
		<ul class="palette-list">
			{#if filtered.length === 0}
				<li class="palette-empty">No agents found</li>
			{/if}
			{#each filtered as item, i (item.name + item.team)}
				<li>
					<button
						class="palette-item"
						class:selected={i === selectedIndex}
						onclick={() => onSelect(item.team, item.name)}
					>
						<span class="palette-emoji">{item.emoji || item.name.slice(0, 1).toUpperCase()}</span>
						<span class="palette-name">{item.name}</span>
						<span class="palette-team">{item.team}</span>
					</button>
				</li>
			{/each}
		</ul>
	</div>
</div>

<style>
	.overlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.6);
		display: flex;
		align-items: flex-start;
		justify-content: center;
		padding-top: 15vh;
		z-index: 100;
	}

	.palette {
		background: #1a2030;
		border: 1px solid #2a3348;
		border-radius: 10px;
		width: 420px;
		max-height: 400px;
		display: flex;
		flex-direction: column;
		overflow: hidden;
		box-shadow: 0 24px 64px rgba(0, 0, 0, 0.5);
	}

	.palette-input {
		padding: 14px 16px;
		background: transparent;
		border: none;
		border-bottom: 1px solid #2a3348;
		color: #e2e8f0;
		font-size: 0.95rem;
		outline: none;
		width: 100%;
		box-sizing: border-box;
	}

	.palette-input::placeholder {
		color: #475569;
	}

	.palette-list {
		list-style: none;
		margin: 0;
		padding: 4px 0;
		overflow-y: auto;
	}

	.palette-empty {
		padding: 12px 16px;
		color: #475569;
		font-size: 0.85rem;
		font-style: italic;
	}

	.palette-list li {
		list-style: none;
	}

	.palette-item {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 9px 16px;
		width: 100%;
		background: transparent;
		border: none;
		color: #e2e8f0;
		font: inherit;
		cursor: pointer;
		text-align: left;
	}

	.palette-item:hover,
	.palette-item.selected {
		background: #1e3a5f;
	}

	.palette-emoji {
		font-size: 1rem;
		width: 24px;
		text-align: center;
		flex-shrink: 0;
	}

	.palette-name {
		flex: 1;
		font-size: 0.875rem;
		font-weight: 500;
	}

	.palette-team {
		font-size: 0.7rem;
		color: #64748b;
	}
</style>
