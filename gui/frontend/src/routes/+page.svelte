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

<div class="flex h-screen overflow-hidden font-sans">
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
	<div class="flex-1 flex flex-col overflow-hidden bg-base-100">
		<!-- Tab bar + daemon status -->
		<header class="flex items-center justify-between px-4 h-11 border-b border-neutral bg-base-200 shrink-0">
			<div class="tabs tabs-box">
				<button
					class="tab"
					class:tab-active={chatStore.activeTab === 'chat'}
					onclick={() => chatStore.setActiveTab('chat')}
				>
					Chat
				</button>
				<button
					class="tab"
					class:tab-active={chatStore.activeTab === 'feed'}
					onclick={() => chatStore.setActiveTab('feed')}
				>
					Agent Feed
				</button>
			</div>
			<span class:text-success={daemonOnline} class:text-error={!daemonOnline} class="text-xs">
				{statusMessage}
			</span>
		</header>

		{#if chatStore.activeTab === 'chat'}
			{#if !chatStore.activeContact}
				<div class="flex-1 flex items-center justify-center text-neutral-content text-sm italic">
					<p>Select an agent from the sidebar</p>
				</div>
			{:else}
				<div
					class="flex-1 overflow-y-auto p-4 pb-2 flex flex-col scroll-smooth"
					bind:this={messagesEl}
					onscroll={handleScroll}
				>
					{#if loadingMore}
						<div class="text-center text-xs text-neutral-content py-1.5">Loading…</div>
					{/if}
					{#if chatStore.messages.length === 0}
						<div class="flex-1 flex items-center justify-center text-neutral-content text-sm italic py-10">No messages yet</div>
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
