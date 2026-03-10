<script lang="ts">
	import type { TeamInfo } from '../../../bindings/github.com/tta-lab/ttal-cli/gui/models.js';

	interface Props {
		teams: TeamInfo[];
		activeTeam: string | null;
		activeContact: string | null;
		avatarMap: Record<string, string>;
		onSelectTeam: (name: string) => void;
		onSelect: (name: string) => void;
	}

	let { teams, activeTeam, activeContact, avatarMap, onSelectTeam, onSelect }: Props = $props();

	let visibleAgents = $derived(teams.find((t) => t.name === activeTeam)?.agents ?? []);
</script>

<aside class="w-56 min-w-44 bg-base-300 border-r border-neutral flex flex-col overflow-hidden">
	{#if teams.length > 1}
		<div class="tabs tabs-box gap-0.5 px-2 pt-2 border-b border-neutral flex-wrap">
			{#each teams as team (team.name)}
				<button
					class="tab tab-sm"
					class:tab-active={team.name === activeTeam}
					onclick={() => onSelectTeam(team.name)}
				>
					{team.name}
				</button>
			{/each}
		</div>
	{/if}

	<ul class="menu menu-sm p-0 py-1.5 overflow-y-auto flex-1">
		{#if visibleAgents.length === 0}
			<li class="px-3.5 py-3 text-xs text-neutral-content italic">No agents in this team</li>
		{/if}
		{#each visibleAgents as agent (agent.name)}
			<li>
				<a
					class="flex items-center gap-2.5 px-3 py-2 rounded-md mx-1.5 cursor-pointer"
					class:active={agent.name === activeContact}
					onclick={() => onSelect(agent.name)}
				>
					<div class="w-8 h-8 rounded-full bg-info/20 text-info flex items-center justify-center text-sm font-semibold shrink-0 overflow-hidden">
						{#if avatarMap[agent.name]}
							<img src={avatarMap[agent.name]} alt={agent.name} class="w-full h-full object-cover" />
						{:else if agent.emoji}
							<span class="text-lg">{agent.emoji}</span>
						{:else}
							{agent.name.slice(0, 1).toUpperCase()}
						{/if}
					</div>
					<div class="flex flex-col min-w-0">
						<span class="text-sm font-medium text-base-content truncate">{agent.name}</span>
						{#if agent.description}
							<span class="text-xs text-neutral-content truncate">{agent.description}</span>
						{/if}
					</div>
				</a>
			</li>
		{/each}
	</ul>
</aside>
