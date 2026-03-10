import type { Message } from '../../../bindings/github.com/tta-lab/ttal-cli/internal/ent/models.js';
import type { Contact, TeamInfo } from '../../../bindings/github.com/tta-lab/ttal-cli/gui/models.js';

// Svelte 5 class-based reactive store.
// Nulls from Wails codegen are filtered at the boundary — consumers get clean Message[].
class ChatStore {
	messages = $state<Message[]>([]);
	activeContact = $state<string | null>(null);
	contacts = $state<Contact[]>([]);
	feedMessages = $state<Message[]>([]);
	activeTab = $state<'chat' | 'feed'>('chat');
	userName = $state('');
	teams = $state<TeamInfo[]>([]);
	activeTeam = $state<string | null>(null);

	setMessages(raw: (Message | null)[]) {
		this.messages = raw.filter((m): m is Message => m != null);
	}

	setFeedMessages(raw: (Message | null)[]) {
		this.feedMessages = raw.filter((m): m is Message => m != null);
	}

	setActiveContact(contact: string | null) {
		this.activeContact = contact;
		this.messages = []; // reset pagination when switching contacts
	}

	setActiveTab(tab: 'chat' | 'feed') {
		this.activeTab = tab;
	}

	setTeams(teams: TeamInfo[]) {
		this.teams = teams;
		if (!this.activeTeam && teams.length > 0) {
			this.activeTeam = teams[0].name;
		}
	}

	setActiveTeam(team: string) {
		this.activeTeam = team;
	}

	loadMore(older: (Message | null)[]) {
		const clean = older.filter((m): m is Message => m != null);
		const existingIds = new Set(this.messages.map((m) => m.id?.toString()));
		const unique = clean.filter((m) => !existingIds.has(m.id?.toString()));
		this.messages = [...unique, ...this.messages];
	}
}

export const chatStore = new ChatStore();
