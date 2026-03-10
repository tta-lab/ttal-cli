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

<div class="input-bar">
	<textarea
		class="msg-input"
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
	<button class="send-btn" onclick={send} disabled={disabled || sending} title="Send (Enter)">
		<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
			<path d="M22 2L11 13" />
			<path d="M22 2L15 22 11 13 2 9l20-7z" />
		</svg>
	</button>
</div>

<style>
	.input-bar {
		display: flex;
		align-items: flex-end;
		gap: 8px;
		padding: 10px 14px;
		border-top: 1px solid #2a3348;
		background: #131d2e;
	}

	.msg-input {
		flex: 1;
		background: #1e2a40;
		border: 1px solid #2a3348;
		border-radius: 10px;
		color: #e2e8f0;
		font-size: 0.875rem;
		line-height: 1.5;
		padding: 8px 12px;
		resize: none;
		outline: none;
		font-family: inherit;
		min-height: 36px;
		max-height: 120px;
		overflow-y: auto;
		transition: border-color 0.15s;
	}

	.msg-input:focus {
		border-color: #3b82f6;
	}

	.msg-input:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.send-btn {
		width: 36px;
		height: 36px;
		border-radius: 50%;
		background: #2563eb;
		border: none;
		color: #fff;
		display: flex;
		align-items: center;
		justify-content: center;
		cursor: pointer;
		flex-shrink: 0;
		transition: background 0.15s;
		padding: 0;
		margin: 0;
		line-height: normal;
	}

	.send-btn:hover:not(:disabled) {
		background: #1d4ed8;
	}

	.send-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.send-btn svg {
		width: 16px;
		height: 16px;
	}
</style>
