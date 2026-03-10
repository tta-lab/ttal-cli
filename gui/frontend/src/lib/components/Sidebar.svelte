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

<aside class="sidebar">
	{#if teams.length > 1}
		<div class="team-tabs">
			{#each teams as team (team.name)}
				<button
					class="team-tab"
					class:active={team.name === activeTeam}
					onclick={() => onSelectTeam(team.name)}
				>
					{team.name}
				</button>
			{/each}
		</div>
	{/if}

	<ul class="agent-list">
		{#if visibleAgents.length === 0}
			<li class="empty-hint">No agents in this team</li>
		{/if}
		{#each visibleAgents as agent (agent.name)}
			<li>
				<button
					class="agent-item"
					class:active={agent.name === activeContact}
					onclick={() => onSelect(agent.name)}
				>
					<div class="agent-avatar">
						{#if avatarMap[agent.name]}
							<img src={avatarMap[agent.name]} alt={agent.name} />
						{:else if agent.emoji}
							<span class="agent-emoji">{agent.emoji}</span>
						{:else}
							{agent.name.slice(0, 1).toUpperCase()}
						{/if}
					</div>
					<div class="agent-info">
						<span class="agent-name">{agent.name}</span>
						{#if agent.description}
							<span class="agent-desc">{agent.description}</span>
						{/if}
					</div>
				</button>
			</li>
		{/each}
	</ul>
</aside>

<style>
	.sidebar {
		width: 220px;
		min-width: 180px;
		background: #1a2030;
		border-right: 1px solid #2a3348;
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}

	.team-tabs {
		display: flex;
		gap: 2px;
		padding: 8px 8px 0;
		border-bottom: 1px solid #2a3348;
		flex-wrap: wrap;
	}

	.team-tab {
		padding: 4px 10px;
		border-radius: 4px 4px 0 0;
		border: none;
		background: transparent;
		color: #64748b;
		font-size: 0.75rem;
		font-weight: 500;
		cursor: pointer;
		transition: all 0.1s;
	}

	.team-tab:hover {
		color: #e2e8f0;
		background: #1e2a40;
	}

	.team-tab.active {
		color: #e2e8f0;
		background: #1e2a40;
		border-bottom: 2px solid #3b82f6;
	}

	.agent-list {
		list-style: none;
		margin: 0;
		padding: 6px 0;
		overflow-y: auto;
		flex: 1;
	}

	.empty-hint {
		padding: 12px 14px;
		font-size: 0.8rem;
		color: #475569;
		font-style: italic;
	}

	.agent-list li {
		list-style: none;
	}

	.agent-item {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 8px 12px;
		cursor: pointer;
		border-radius: 6px;
		margin: 2px 6px;
		transition: background 0.1s;
		background: transparent;
		border: none;
		width: calc(100% - 12px);
		text-align: left;
		color: inherit;
		font: inherit;
	}

	.agent-item:hover {
		background: #1e2a40;
	}

	.agent-item.active {
		background: #1e3a5f;
	}

	.agent-avatar {
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

	.agent-avatar img {
		width: 100%;
		height: 100%;
		object-fit: cover;
	}

	.agent-emoji {
		font-size: 1.1rem;
	}

	.agent-info {
		display: flex;
		flex-direction: column;
		min-width: 0;
	}

	.agent-name {
		font-size: 0.875rem;
		font-weight: 500;
		color: #e2e8f0;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.agent-desc {
		font-size: 0.7rem;
		color: #475569;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
</style>
