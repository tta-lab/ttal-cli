import type { Message } from '../../../bindings/github.com/tta-lab/ttal-cli/internal/ent/models.js';
import type { Contact } from '../../../bindings/github.com/tta-lab/ttal-cli/gui/models.js';

// Svelte 5 class-based reactive store.
// Class fields with $state() are deeply reactive signals — safe to import
// and use as `chatStore.messages` in any .svelte or .svelte.ts file.
class ChatStore {
	messages = $state<(Message | null)[]>([]);
	activeContact = $state<string | null>(null);
	contacts = $state<Contact[]>([]);
	feedMessages = $state<(Message | null)[]>([]);
	activeTab = $state<'chat' | 'feed'>('chat');
	offset = $state(0);
	userName = $state('');

	// Avatar blob URL cache — keyed by agent name
	avatarCache = new Map<string, string>();

	setActiveContact(contact: string | null) {
		this.activeContact = contact;
		this.offset = 0; // reset pagination when switching contacts
		this.messages = [];
	}

	setActiveTab(tab: 'chat' | 'feed') {
		this.activeTab = tab;
	}

	loadMore(older: (Message | null)[]) {
		const existingIds = new Set(
			this.messages.filter(Boolean).map((m) => m!.id?.toString())
		);
		const unique = older.filter((m) => m && !existingIds.has(m.id?.toString()));
		this.messages = [...unique, ...this.messages];
		this.offset += older.length;
	}

	cacheAvatar(name: string, url: string) {
		this.avatarCache.set(name, url);
	}

	revokeAvatars() {
		for (const url of this.avatarCache.values()) {
			URL.revokeObjectURL(url);
		}
		this.avatarCache.clear();
	}
}

export const chatStore = new ChatStore();
