/** Format a timestamp as a human-friendly relative or clock string. */
export function formatTime(ts: unknown): string {
	if (!ts) return '';
	const d = new Date(ts as string);
	if (isNaN(d.getTime())) return '';
	return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

/** Format a timestamp as relative time for the sidebar (e.g. "5m ago"). */
export function formatRelativeTime(ts: unknown): string {
	if (!ts) return '';
	const d = new Date(ts as string);
	if (isNaN(d.getTime())) return '';
	const now = new Date();
	const diffMins = Math.floor((now.getTime() - d.getTime()) / 60000);
	if (diffMins < 1) return 'just now';
	if (diffMins < 60) return `${diffMins}m ago`;
	const diffHours = Math.floor(diffMins / 60);
	if (diffHours < 24) return `${diffHours}h ago`;
	return d.toLocaleDateString();
}
