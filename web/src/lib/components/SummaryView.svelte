<script lang="ts">
	import { marked } from 'marked';
	import { exportToBlog, listMessages, streamChat, type ExportResult, type ChatMessage } from '$lib/api';

	interface Props {
		summaryId: string | null;
		videoUrl: string;
		videoTitle: string;
		summaryText: string;
		status: string;
		streaming: boolean;
	}

	let { summaryId, videoUrl, videoTitle, summaryText, status, streaming }: Props = $props();

	marked.setOptions({
		breaks: true,
		gfm: true
	});

	let renderedHtml = $derived.by(() => {
		if (!summaryText) return '';
		return marked.parse(summaryText) as string;
	});

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

	let exporting = $state(false);
	let exportResult = $state<ExportResult | null>(null);
	let exportError = $state('');

	// ── Chat state ────────────────────────────────────────────────────────────
	let chatMessages = $state<ChatMessage[]>([]);
	let chatInput = $state('');
	let chatStreaming = $state(false);
	let chatStreamText = $state('');
	let cancelChat: (() => void) | null = null;

	// Load/reset chat history when summaryId changes
	$effect(() => {
		const sid = summaryId;
		chatMessages = [];
		chatStreamText = '';
		if (sid && !streaming) {
			listMessages(sid).then(msgs => {
				// Skip the seeded initial assistant message (index 0) — summary is already rendered above
				chatMessages = msgs.slice(1);
			}).catch(() => {});
		}
	});

	// Reset chat when a new summary starts streaming
	$effect(() => {
		if (streaming) {
			chatMessages = [];
			chatStreamText = '';
		}
	});

	function handleChatKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			sendChat();
		}
	}

	function sendChat() {
		if (!chatInput.trim() || chatStreaming || !summaryId) return;

		const userText = chatInput.trim();
		chatInput = '';
		chatStreaming = true;
		chatStreamText = '';

		// Optimistically add the user bubble
		chatMessages = [...chatMessages, {
			id: crypto.randomUUID(),
			summary_id: summaryId,
			role: 'user',
			content: userText,
			created_at: new Date().toISOString()
		}];

		cancelChat = streamChat(summaryId, userText, {
			onToken(token) {
				chatStreamText += token;
			},
			onDone() {
				chatMessages = [...chatMessages, {
					id: crypto.randomUUID(),
					summary_id: summaryId!,
					role: 'assistant',
					content: chatStreamText,
					created_at: new Date().toISOString()
				}];
				chatStreamText = '';
				chatStreaming = false;
				cancelChat = null;
			},
			onError(msg) {
				chatStreamText = '';
				chatStreaming = false;
				cancelChat = null;
				// Re-append error as assistant bubble
				chatMessages = [...chatMessages, {
					id: crypto.randomUUID(),
					summary_id: summaryId!,
					role: 'assistant',
					content: `Error: ${msg}`,
					created_at: new Date().toISOString()
				}];
			}
		});
	}

	async function handleExport() {
		exporting = true;
		exportResult = null;
		exportError = '';
		try {
			exportResult = await exportToBlog(summaryText, videoTitle, videoUrl);
		} catch (err: any) {
			exportError = err.message || 'Export failed';
		} finally {
			exporting = false;
		}
	}
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

		{#if !streaming}
			<div class="export-area">
				{#if exportResult}
					<div class="export-success">
						Published to blog —
						<a href={exportResult.html_url} target="_blank" rel="noopener noreferrer">view on GitHub</a>
					</div>
				{:else}
					<button class="export-btn" onclick={handleExport} disabled={exporting}>
						{#if exporting}
							Publishing…
						{:else}
							Export to Blog
						{/if}
					</button>
					{#if exportError}
						<span class="export-error">{exportError}</span>
					{/if}
				{/if}
			</div>

			<div class="chat-section">
				<div class="chat-divider">
					<span>Ask a follow-up question</span>
				</div>

				{#if chatMessages.length > 0 || chatStreamText}
					<div class="chat-thread">
						{#each chatMessages as msg (msg.id)}
							<div class="chat-bubble" class:user={msg.role === 'user'} class:assistant={msg.role === 'assistant'}>
								{#if msg.role === 'assistant'}
									{@html marked.parse(msg.content) as string}
								{:else}
									{msg.content}
								{/if}
							</div>
						{/each}
						{#if chatStreamText}
							<div class="chat-bubble assistant">
								{@html marked.parse(chatStreamText) as string}<span class="cursor-blink">▌</span>
							</div>
						{/if}
					</div>
				{/if}

				<div class="chat-input-row">
					<textarea
						class="chat-textarea"
						placeholder="Ask something about this video…"
						bind:value={chatInput}
						onkeydown={handleChatKeydown}
						disabled={chatStreaming}
						rows="2"
					></textarea>
					<button class="chat-send-btn" onclick={sendChat} disabled={chatStreaming || !chatInput.trim()}>
						{#if chatStreaming}
							<span class="spinner"></span>
						{:else}
							<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg>
						{/if}
					</button>
				</div>
			</div>
		{/if}
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

	.export-area {
		display: flex;
		align-items: center;
		gap: 12px;
		margin-top: 20px;
		padding-top: 16px;
		border-top: 1px solid var(--border);
	}

	.export-btn {
		padding: 8px 16px;
		background: var(--accent-hover);
		color: var(--ash-grey);
		border-radius: var(--radius-sm);
		font-size: 0.85rem;
		font-weight: 500;
		transition: background 0.15s, color 0.15s;
	}

	.export-btn:hover:not(:disabled) {
		background: var(--accent);
		color: var(--charcoal-blue);
	}

	.export-btn:disabled {
		opacity: 0.6;
		cursor: default;
	}

	.export-success {
		font-size: 0.85rem;
		color: var(--text-muted);
	}

	.export-success a {
		color: var(--accent);
		text-decoration: underline;
	}

	.export-error {
		font-size: 0.82rem;
		color: #e57373;
	}

	.chat-section {
		margin-top: 28px;
	}

	.chat-divider {
		display: flex;
		align-items: center;
		gap: 12px;
		margin-bottom: 16px;
		color: var(--text-muted);
		font-size: 0.78rem;
		text-transform: uppercase;
		letter-spacing: 0.06em;
	}

	.chat-divider::before,
	.chat-divider::after {
		content: '';
		flex: 1;
		height: 1px;
		background: var(--border);
	}

	.chat-thread {
		display: flex;
		flex-direction: column;
		gap: 10px;
		margin-bottom: 16px;
	}

	.chat-bubble {
		max-width: 88%;
		padding: 10px 14px;
		border-radius: var(--radius);
		font-size: 0.9rem;
		line-height: 1.55;
	}

	.chat-bubble.user {
		align-self: flex-end;
		background: var(--accent-hover);
		color: var(--ash-grey);
		border-bottom-right-radius: 4px;
	}

	.chat-bubble.assistant {
		align-self: flex-start;
		background: var(--bg-sidebar);
		color: var(--text);
		border-bottom-left-radius: 4px;
	}

	.chat-bubble.assistant :global(p) { margin-bottom: 0.5em; }
	.chat-bubble.assistant :global(p:last-child) { margin-bottom: 0; }
	.chat-bubble.assistant :global(ul),
	.chat-bubble.assistant :global(ol) { padding-left: 1.4em; margin-bottom: 0.5em; }

	.chat-input-row {
		display: flex;
		gap: 8px;
		align-items: flex-end;
	}

	.chat-textarea {
		flex: 1;
		padding: 10px 14px;
		background: var(--bg-input);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		font-size: 0.9rem;
		font-family: inherit;
		resize: none;
		outline: none;
		transition: border-color 0.15s;
		color: var(--text);
	}

	.chat-textarea:focus {
		border-color: var(--accent);
	}

	.chat-textarea::placeholder {
		color: var(--text-muted);
	}

	.chat-send-btn {
		width: 42px;
		height: 42px;
		flex-shrink: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--accent);
		color: var(--charcoal-blue);
		border-radius: var(--radius);
		transition: background 0.15s, opacity 0.15s;
	}

	.chat-send-btn:hover:not(:disabled) {
		background: var(--deep-teal);
		color: var(--ash-grey);
	}

	.chat-send-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.spinner {
		width: 16px;
		height: 16px;
		border: 2px solid transparent;
		border-top-color: currentColor;
		border-radius: 50%;
		animation: spin 0.6s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}
</style>
