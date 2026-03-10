<script lang="ts">
	import type { Contact } from '../../../bindings/github.com/tta-lab/ttal-cli/gui/models.js';

	interface Props {
		contacts: Contact[];
		activeContact: string | null;
		onSelect: (name: string) => void;
	}

	let { contacts, activeContact, onSelect }: Props = $props();

	function formatTime(ts: unknown): string {
		if (!ts) return '';
		const d = new Date(ts as string);
		if (isNaN(d.getTime())) return '';
		const now = new Date();
		const diffMs = now.getTime() - d.getTime();
		const diffMins = Math.floor(diffMs / 60000);
		if (diffMins < 1) return 'just now';
		if (diffMins < 60) return `${diffMins}m ago`;
		const diffHours = Math.floor(diffMins / 60);
		if (diffHours < 24) return `${diffHours}h ago`;
		return d.toLocaleDateString();
	}
</script>

<aside class="sidebar">
	<div class="sidebar-header">
		<span class="sidebar-title">Conversations</span>
	</div>
	<ul class="contact-list">
		{#if contacts.length === 0}
			<li class="empty-hint">No conversations yet</li>
		{/if}
		{#each contacts as contact (contact.name)}
			<li>
				<button
					class="contact-item"
					class:active={contact.name === activeContact}
					onclick={() => onSelect(contact.name)}
				>
					<div class="contact-avatar">
						{contact.name.slice(0, 1).toUpperCase()}
					</div>
					<div class="contact-info">
						<span class="contact-name">{contact.name}</span>
						<span class="contact-time">{formatTime(contact.lastMessageAt)}</span>
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

	.sidebar-header {
		padding: 16px 12px 10px;
		border-bottom: 1px solid #2a3348;
	}

	.sidebar-title {
		font-size: 0.75rem;
		font-weight: 600;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: #64748b;
	}

	.contact-list {
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

	.contact-list li {
		list-style: none;
	}

	.contact-item {
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

	.contact-item:hover {
		background: #1e2a40;
	}

	.contact-item.active {
		background: #1e3a5f;
	}

	.contact-avatar {
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
	}

	.contact-info {
		display: flex;
		flex-direction: column;
		min-width: 0;
	}

	.contact-name {
		font-size: 0.875rem;
		font-weight: 500;
		color: #e2e8f0;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.contact-time {
		font-size: 0.7rem;
		color: #475569;
	}
</style>
