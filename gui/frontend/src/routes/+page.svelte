<script lang="ts">
	import { onDestroy, tick } from 'svelte';
	import * as ChatService from '../../bindings/github.com/tta-lab/ttal-cli/gui/chatservice.js';
	import { chatStore } from '$lib/stores/messages.svelte.js';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import ChatBubble from '$lib/components/ChatBubble.svelte';
	import MessageInput from '$lib/components/MessageInput.svelte';
	import AgentFeed from '$lib/components/AgentFeed.svelte';
	import CommandPalette from '$lib/components/CommandPalette.svelte';

	let daemonOnline = $state(false);
	let statusMessage = $state('Connecting…');
	let messagesEl = $state<HTMLElement | null>(null);
	let loadingMore = $state(false);
	let showCommandPalette = $state(false);

	// Avatar blob URL cache — keyed by agent name, cleaned up on destroy
	const avatarCache = new Map<string, string>();
	let avatarMap = $state<Record<string, string>>({});

	async function getAvatarUrl(name: string): Promise<string> {
		if (avatarCache.has(name)) return avatarCache.get(name)!;
		try {
			const url = await ChatService.GetAvatar(name);
			avatarCache.set(name, url);
			return url;
		} catch (err) {
			console.warn(`GetAvatar failed for "${name}":`, err);
			// Cache empty string so we don't retry on every contact switch
			avatarCache.set(name, '');
			return ''; // fallback to emoji/initials
		}
	}

	async function refreshAvatars() {
		if (!chatStore.activeContact) return;
		const url = await getAvatarUrl(chatStore.activeContact);
		if (url) avatarMap = { ...avatarMap, [chatStore.activeContact]: url };
	}

	// Pre-fetch avatars for all agents in the active team
	async function prefetchTeamAvatars() {
		const team = chatStore.teams.find((t) => t.name === chatStore.activeTeam);
		if (!team) return;
		for (const agent of team.agents) {
			if (!avatarCache.has(agent.name)) {
				const url = await getAvatarUrl(agent.name);
				if (url) avatarMap = { ...avatarMap, [agent.name]: url };
			}
		}
	}

	// Initialize: user name, daemon status, teams, contacts
	async function init() {
		try {
			chatStore.userName = await ChatService.GetUserName();
		} catch (err) {
			statusMessage = `Error loading user: ${err}`;
			return;
		}
		try {
			daemonOnline = await ChatService.IsDaemonRunning();
			statusMessage = daemonOnline ? 'Daemon online' : 'Daemon offline — read-only mode';
		} catch {
			daemonOnline = false;
			statusMessage = 'Daemon offline — read-only mode';
		}
		try {
			const teams = await ChatService.GetTeams();
			chatStore.setTeams(teams);
			prefetchTeamAvatars().catch((err) => console.warn('prefetchTeamAvatars failed:', err));
		} catch (err) {
			console.error('GetTeams failed:', err);
			statusMessage = `Failed to load teams: ${err}`;
		}
		try {
			chatStore.contacts = await ChatService.GetContacts();
		} catch (err) {
			console.error('GetContacts failed during init:', err);
		}
	}

	init();

	// Poll contacts every 5s — they change rarely
	const contactsInterval = setInterval(async () => {
		try {
			chatStore.contacts = await ChatService.GetContacts();
		} catch (err) {
			console.error('GetContacts poll failed:', err);
		}
	}, 5000);

	// Poll messages for active contact every 500ms
	let messagesInterval: ReturnType<typeof setInterval> | null = null;
	let consecutiveMessageFailures = 0;

	$effect(() => {
		const contact = chatStore.activeContact;

		if (messagesInterval) {
			clearInterval(messagesInterval);
			messagesInterval = null;
		}
		consecutiveMessageFailures = 0;
		if (!contact) return;

		refreshAvatars();

		messagesInterval = setInterval(async () => {
			if (!chatStore.activeContact) return;
			try {
				const msgs = await ChatService.GetMessages(
					chatStore.userName,
					chatStore.activeContact,
					50,
					0
				);
				consecutiveMessageFailures = 0;
				const prevCount = chatStore.messages.length;
				chatStore.setMessages(msgs);
				if (chatStore.messages.length > prevCount) {
					await tick();
					scrollToBottom();
				}
			} catch (err) {
				consecutiveMessageFailures++;
				console.error('GetMessages poll failed:', err);
				if (consecutiveMessageFailures >= 3) {
					statusMessage = 'Connection lost — retrying…';
					daemonOnline = false;
				}
			}
		}, 500);

		return () => {
			if (messagesInterval) {
				clearInterval(messagesInterval);
				messagesInterval = null;
			}
		};
	});

	// Prefetch avatars when active team changes
	$effect(() => {
		chatStore.activeTeam;
		prefetchTeamAvatars().catch((err) => console.warn('prefetchTeamAvatars failed:', err));
	});

	// Poll agent feed every 500ms when feed tab is active
	let feedInterval: ReturnType<typeof setInterval> | null = null;

	$effect(() => {
		const tab = chatStore.activeTab;

		if (feedInterval) {
			clearInterval(feedInterval);
			feedInterval = null;
		}
		if (tab !== 'feed') return;

		feedInterval = setInterval(async () => {
			try {
				chatStore.setFeedMessages(await ChatService.GetAgentFeedMessages(50, 0));
			} catch (err) {
				console.error('GetAgentFeedMessages poll failed:', err);
			}
		}, 500);

		return () => {
			if (feedInterval) {
				clearInterval(feedInterval);
				feedInterval = null;
			}
		};
	});

	function scrollToBottom() {
		if (messagesEl) {
			messagesEl.scrollTop = messagesEl.scrollHeight;
		}
	}

	async function handleScroll() {
		if (!messagesEl || loadingMore || !chatStore.activeContact) return;
		if (messagesEl.scrollTop > 40) return;
		loadingMore = true;
		try {
			const older = await ChatService.GetMessages(
				chatStore.userName,
				chatStore.activeContact,
				50,
				chatStore.messages.length
			);
			if (older.length > 0) {
				const prevScrollHeight = messagesEl.scrollHeight;
				chatStore.loadMore(older);
				await tick();
				messagesEl.scrollTop = messagesEl.scrollHeight - prevScrollHeight;
			}
		} catch (err) {
			console.error('Load more messages failed:', err);
		} finally {
			loadingMore = false;
		}
	}

	async function handleSend(content: string) {
		if (!chatStore.activeContact) return;
		try {
			await ChatService.SendMessage(chatStore.activeContact, content);
		} catch (err) {
			console.error('SendMessage failed:', err);
			throw err;
		}
	}

	function currentTeamAgents() {
		const team = chatStore.teams.find((t) => t.name === chatStore.activeTeam);
		return team?.agents ?? [];
	}

	function selectAgent(delta: 1 | -1) {
		const agents = currentTeamAgents();
		if (!agents.length) return;
		const idx = agents.findIndex((a) => a.name === chatStore.activeContact);
		const next = (idx + delta + agents.length) % agents.length;
		chatStore.setActiveContact(agents[next].name);
	}

	function handleGlobalKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && showCommandPalette) {
			showCommandPalette = false;
			return;
		}
		if (!e.metaKey) return;
		if (e.key === 'ArrowUp') { e.preventDefault(); selectAgent(-1); }
		else if (e.key === 'ArrowDown') { e.preventDefault(); selectAgent(1); }
		else if (e.key === 'k') { e.preventDefault(); showCommandPalette = !showCommandPalette; }
	}

	function handlePaletteSelect(team: string, agent: string) {
		chatStore.setActiveTeam(team);
		chatStore.setActiveContact(agent);
		showCommandPalette = false;
	}

	onDestroy(() => {
		clearInterval(contactsInterval);
		if (messagesInterval) clearInterval(messagesInterval);
		if (feedInterval) clearInterval(feedInterval);
		avatarCache.clear();
	});
</script>

<svelte:window onkeydown={handleGlobalKeydown} />

{#if showCommandPalette}
	<CommandPalette
		teams={chatStore.teams}
		onSelect={handlePaletteSelect}
		onClose={() => (showCommandPalette = false)}
	/>
{/if}

<div class="app-shell">
	<!-- Sidebar -->
	<Sidebar
		teams={chatStore.teams}
		activeTeam={chatStore.activeTeam}
		activeContact={chatStore.activeContact}
		{avatarMap}
		onSelectTeam={(name) => chatStore.setActiveTeam(name)}
		onSelect={(name) => chatStore.setActiveContact(name)}
	/>

	<!-- Main area -->
	<div class="main">
		<!-- Tab bar + daemon status -->
		<header class="topbar">
			<div class="tabs">
				<button
					class="tab"
					class:active={chatStore.activeTab === 'chat'}
					onclick={() => chatStore.setActiveTab('chat')}
				>
					Chat
				</button>
				<button
					class="tab"
					class:active={chatStore.activeTab === 'feed'}
					onclick={() => chatStore.setActiveTab('feed')}
				>
					Agent Feed
				</button>
			</div>
			<span class="status" class:online={daemonOnline} class:offline={!daemonOnline}>
				{statusMessage}
			</span>
		</header>

		{#if chatStore.activeTab === 'chat'}
			{#if !chatStore.activeContact}
				<div class="empty-state">
					<p>Select an agent from the sidebar</p>
				</div>
			{:else}
				<div
					class="messages-area"
					bind:this={messagesEl}
					onscroll={handleScroll}
				>
					{#if loadingMore}
						<div class="loading-more">Loading…</div>
					{/if}
					{#if chatStore.messages.length === 0}
						<div class="no-messages">No messages yet</div>
					{/if}
					{#each chatStore.messages as msg, i (msg.id?.toString() ?? `idx-${i}`)}
						<ChatBubble
							message={msg}
							userName={chatStore.userName}
							avatarUrl={avatarMap[msg.sender ?? ''] ?? ''}
						/>
					{/each}
				</div>

				<MessageInput
					onSend={handleSend}
					disabled={!daemonOnline}
					placeholder={daemonOnline
						? `Message ${chatStore.activeContact}…`
						: 'Daemon offline — read-only'}
				/>
			{/if}
		{:else}
			<AgentFeed messages={chatStore.feedMessages} />
		{/if}
	</div>
</div>

<style>
	:global(html, body) {
		margin: 0;
		padding: 0;
		height: 100%;
		overflow: hidden;
		background: #0f172a;
		color: #e2e8f0;
	}

	:global(#svelte) {
		height: 100%;
	}

	.app-shell {
		display: flex;
		height: 100vh;
		overflow: hidden;
		font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
	}

	.main {
		flex: 1;
		display: flex;
		flex-direction: column;
		overflow: hidden;
		background: #0f172a;
	}

	.topbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0 16px;
		height: 44px;
		border-bottom: 1px solid #2a3348;
		background: #131d2e;
		flex-shrink: 0;
	}

	.tabs {
		display: flex;
		gap: 4px;
	}

	.tab {
		padding: 5px 14px;
		border-radius: 6px;
		border: none;
		background: transparent;
		color: #64748b;
		font-size: 0.85rem;
		cursor: pointer;
		transition: all 0.15s;
		width: auto;
		height: auto;
		line-height: normal;
	}

	.tab:hover {
		color: #e2e8f0;
		background: #1e2a40;
	}

	.tab.active {
		color: #e2e8f0;
		background: #1e2a40;
	}

	.status {
		font-size: 0.75rem;
	}

	.status.online {
		color: #4ade80;
	}

	.status.offline {
		color: #f87171;
	}

	.empty-state {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		color: #475569;
		font-size: 0.9rem;
		font-style: italic;
	}

	.messages-area {
		flex: 1;
		overflow-y: auto;
		padding: 16px 16px 8px;
		display: flex;
		flex-direction: column;
		scroll-behavior: smooth;
	}

	.no-messages {
		flex: 1;
		display: flex;
		align-items: center;
		justify-content: center;
		color: #475569;
		font-size: 0.85rem;
		font-style: italic;
		padding: 40px 0;
	}

	.loading-more {
		text-align: center;
		font-size: 0.75rem;
		color: #475569;
		padding: 6px 0;
	}
</style>
