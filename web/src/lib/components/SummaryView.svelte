<script lang="ts">
	import { marked } from 'marked';

	interface Props {
		videoUrl: string;
		videoTitle: string;
		summaryText: string;
		status: string;
		streaming: boolean;
	}

	let { videoUrl, videoTitle, summaryText, status, streaming }: Props = $props();

	let renderedHtml = $derived(marked.parse(summaryText, { async: false }) as string);

	function extractVideoId(url: string): string | null {
		try {
			const parsed = new URL(url);
			if (parsed.hostname.includes('youtube.com')) {
				return parsed.searchParams.get('v');
			}
			if (parsed.hostname === 'youtu.be') {
				return parsed.pathname.slice(1);
			}
		} catch {}
		return null;
	}

	let videoId = $derived(extractVideoId(videoUrl));
</script>

<div class="summary-view" class:visible={!!videoUrl}>
	{#if videoId}
		<div class="video-embed">
			<iframe
				src="https://www.youtube.com/embed/{videoId}"
				title={videoTitle || 'YouTube Video'}
				frameborder="0"
				allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
				allowfullscreen
			></iframe>
		</div>
	{/if}

	{#if videoTitle}
		<h2 class="video-title">{videoTitle}</h2>
	{/if}

	{#if status && !summaryText}
		<div class="status-msg">
			<span class="status-dot"></span>
			{status}
		</div>
	{/if}

	{#if summaryText}
		<div class="summary-content">
			{@html renderedHtml}
			{#if streaming}
				<span class="cursor-blink">▌</span>
			{/if}
		</div>
	{/if}
</div>

<style>
	.summary-view {
		width: 100%;
		max-width: 720px;
		margin: 0 auto;
		padding-top: 24px;
		opacity: 0;
		transform: translateY(12px);
		transition: opacity 0.4s ease, transform 0.4s ease;
	}

	.summary-view.visible {
		opacity: 1;
		transform: translateY(0);
	}

	.video-embed {
		position: relative;
		width: 100%;
		padding-bottom: 56.25%;
		border-radius: var(--radius);
		overflow: hidden;
		margin-bottom: 16px;
	}

	.video-embed iframe {
		position: absolute;
		top: 0;
		left: 0;
		width: 100%;
		height: 100%;
	}

	.video-title {
		font-size: 1.15rem;
		font-weight: 600;
		color: var(--text);
		margin-bottom: 16px;
	}

	.status-msg {
		display: flex;
		align-items: center;
		gap: 8px;
		color: var(--text-muted);
		font-size: 0.9rem;
		padding: 12px 0;
	}

	.status-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: var(--accent);
		animation: pulse 1.2s ease-in-out infinite;
	}

	@keyframes pulse {
		0%, 100% { opacity: 0.4; }
		50% { opacity: 1; }
	}

	.summary-content {
		line-height: 1.7;
		font-size: 0.95rem;
		color: var(--text);
	}

	.summary-content :global(h1),
	.summary-content :global(h2),
	.summary-content :global(h3) {
		margin-top: 1.2em;
		margin-bottom: 0.5em;
		color: var(--ash-grey);
	}

	.summary-content :global(h1) { font-size: 1.3rem; }
	.summary-content :global(h2) { font-size: 1.15rem; }
	.summary-content :global(h3) { font-size: 1.05rem; }

	.summary-content :global(p) {
		margin-bottom: 0.8em;
	}

	.summary-content :global(ul),
	.summary-content :global(ol) {
		padding-left: 1.5em;
		margin-bottom: 0.8em;
	}

	.summary-content :global(li) {
		margin-bottom: 0.3em;
	}

	.summary-content :global(strong) {
		color: var(--ash-grey);
	}

	.summary-content :global(code) {
		background: var(--bg-sidebar);
		padding: 2px 6px;
		border-radius: 4px;
		font-size: 0.88em;
	}

	.summary-content :global(blockquote) {
		border-left: 3px solid var(--accent);
		padding-left: 12px;
		color: var(--text-muted);
		margin: 0.8em 0;
	}

	.cursor-blink {
		color: var(--accent);
		animation: blink 0.8s step-end infinite;
	}

	@keyframes blink {
		50% { opacity: 0; }
	}
</style>
