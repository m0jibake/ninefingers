<script lang="ts">
	import type { Summary } from '$lib/api';

	interface Props {
		summaries: Summary[];
		activeSummaryId: string | null;
		collapsed: boolean;
		onSelect: (summary: Summary) => void;
		onDelete: (id: string) => void;
		onNewChat: () => void;
		onToggle: () => void;
	}

	let { summaries, activeSummaryId, collapsed, onSelect, onDelete, onNewChat, onToggle }: Props = $props();

	function formatDate(dateStr: string): string {
		const date = new Date(dateStr);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

		if (diffDays === 0) return 'Today';
		if (diffDays === 1) return 'Yesterday';
		if (diffDays < 7) return `${diffDays} days ago`;
		return date.toLocaleDateString();
	}
</script>

<aside class="sidebar" class:collapsed>
	<div class="sidebar-header">
		{#if !collapsed}
			<button class="new-chat-btn" onclick={onNewChat}>
				<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
				New summary
			</button>
		{/if}
		<button class="toggle-btn" onclick={onToggle} aria-label="Toggle sidebar">
			<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
				{#if collapsed}
					<polyline points="9 18 15 12 9 6"/>
				{:else}
					<polyline points="15 18 9 12 15 6"/>
				{/if}
			</svg>
		</button>
	</div>

	{#if !collapsed}
		<nav class="history-list">
			{#each summaries as summary (summary.id)}
				<div
					class="history-item"
					class:active={activeSummaryId === summary.id}
					onclick={() => onSelect(summary)}
					onkeydown={(e: KeyboardEvent) => { if (e.key === 'Enter') onSelect(summary); }}
					role="button"
					tabindex="0"
				>
					<span class="history-title">{summary.video_title || 'Untitled'}</span>
					<span class="history-date">{formatDate(summary.created_at)}</span>
					<button
						class="delete-btn"
						onclick={(e: MouseEvent) => { e.stopPropagation(); onDelete(summary.id); }}
						aria-label="Delete"
					>
						<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
					</button>
				</div>
			{/each}

			{#if summaries.length === 0}
				<p class="empty-state">No summaries yet</p>
			{/if}
		</nav>
	{/if}
</aside>

<style>
	.sidebar {
		width: 280px;
		min-width: 280px;
		height: 100%;
		background: var(--bg-sidebar);
		border-right: 1px solid var(--border);
		display: flex;
		flex-direction: column;
		transition: width 0.2s ease, min-width 0.2s ease;
		overflow: hidden;
	}

	.sidebar.collapsed {
		width: 48px;
		min-width: 48px;
	}

	.sidebar-header {
		padding: 12px;
		display: flex;
		align-items: center;
		gap: 8px;
		border-bottom: 1px solid var(--border);
	}

	.new-chat-btn {
		flex: 1;
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 12px;
		background: var(--accent-hover);
		color: var(--ash-grey);
		border-radius: var(--radius-sm);
		font-size: 0.85rem;
		font-weight: 500;
		transition: background 0.15s;
	}

	.new-chat-btn:hover {
		background: var(--accent);
		color: var(--charcoal-blue);
	}

	.toggle-btn {
		padding: 6px;
		border-radius: var(--radius-sm);
		color: var(--text-muted);
		transition: color 0.15s, background 0.15s;
	}

	.toggle-btn:hover {
		color: var(--text);
		background: var(--bg-hover);
	}

	.history-list {
		flex: 1;
		overflow-y: auto;
		padding: 8px;
	}

	.history-item {
		width: 100%;
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		padding: 10px 12px;
		border-radius: var(--radius-sm);
		text-align: left;
		position: relative;
		transition: background 0.15s;
		margin-bottom: 2px;
	}

	.history-item:hover {
		background: var(--bg-hover);
	}

	.history-item.active {
		background: var(--bg-hover);
	}

	.history-title {
		font-size: 0.85rem;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		width: 100%;
		padding-right: 20px;
	}

	.history-date {
		font-size: 0.72rem;
		color: var(--text-muted);
		margin-top: 2px;
	}

	.delete-btn {
		position: absolute;
		top: 8px;
		right: 8px;
		padding: 4px;
		border-radius: 4px;
		color: var(--text-muted);
		opacity: 0;
		transition: opacity 0.15s, color 0.15s;
	}

	.history-item:hover .delete-btn {
		opacity: 1;
	}

	.delete-btn:hover {
		color: #e57373;
	}

	.empty-state {
		text-align: center;
		color: var(--text-muted);
		font-size: 0.85rem;
		padding: 24px 12px;
	}
</style>
