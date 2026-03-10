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
<div class="modal modal-open">
	<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
	<div class="modal-backdrop" onclick={onClose}></div>
	<div class="modal-box bg-base-300 border border-neutral w-[420px] max-h-[400px] p-0 mt-[15vh] overflow-hidden">
		<input
			class="input input-ghost w-full border-b border-neutral rounded-none text-base-content px-4 py-3.5"
			placeholder="Search agents…"
			bind:value={query}
			onkeydown={handleKeydown}
			autofocus
		/>
		<ul class="menu menu-sm p-1 overflow-y-auto">
			{#if filtered.length === 0}
				<li class="px-4 py-3 text-neutral-content text-sm italic">No agents found</li>
			{/if}
			{#each filtered as item, i (item.name + item.team)}
				<li>
					<button
						class="flex items-center gap-2.5"
						class:active={i === selectedIndex}
						onclick={() => onSelect(item.team, item.name)}
					>
						<span class="text-base w-6 text-center shrink-0">{item.emoji || item.name.slice(0, 1).toUpperCase()}</span>
						<span class="flex-1 text-sm font-medium">{item.name}</span>
						<span class="text-xs text-neutral-content">{item.team}</span>
					</button>
				</li>
			{/each}
		</ul>
	</div>
</div>
