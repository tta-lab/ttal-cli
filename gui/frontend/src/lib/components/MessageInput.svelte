<script lang="ts">
	interface Props {
		onSend: (content: string) => void | Promise<void>;
		disabled?: boolean;
		placeholder?: string;
	}

	let { onSend, disabled = false, placeholder = 'Type a message…' }: Props = $props();

	let value = $state('');
	let sending = $state(false);

	async function send() {
		const trimmed = value.trim();
		if (!trimmed || disabled || sending) return;
		sending = true;
		const saved = value;
		value = '';
		try {
			await onSend(trimmed);
		} catch {
			// Restore message so the user doesn't lose what they typed
			value = saved;
		} finally {
			sending = false;
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			send();
		}
	}
</script>

<div class="flex items-end gap-2 p-2.5 px-3.5 border-t border-neutral bg-base-200">
	<textarea
		class="textarea textarea-bordered flex-1 bg-secondary border-neutral text-base-content text-sm leading-relaxed resize-none min-h-9 max-h-30 overflow-y-auto focus:border-primary"
		bind:value
		{placeholder}
		{disabled}
		rows="1"
		onkeydown={handleKeydown}
		oninput={(e) => {
			// Auto-grow: reset then expand
			const el = e.currentTarget as HTMLTextAreaElement;
			el.style.height = 'auto';
			el.style.height = Math.min(el.scrollHeight, 120) + 'px';
		}}
	></textarea>
	<button class="btn btn-primary btn-circle btn-sm" onclick={send} disabled={disabled || sending} title="Send (Enter)">
		<svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
			<path d="M22 2L11 13" />
			<path d="M22 2L15 22 11 13 2 9l20-7z" />
		</svg>
	</button>
</div>
