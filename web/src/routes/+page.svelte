<script lang="ts">
	import { onMount } from 'svelte';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import InputArea from '$lib/components/InputArea.svelte';
	import SummaryView from '$lib/components/SummaryView.svelte';
	import { listSummaries, deleteSummary, streamSummarize, type Summary } from '$lib/api';

	let summaries = $state<Summary[]>([]);
	let activeSummaryId = $state<string | null>(null);
	let sidebarCollapsed = $state(false);

	// Input state
	let url = $state('');
	let model = $state('z-ai/glm4.7');
	let language = $state('en');
	let prompt = $state('Give me a thorough summary of this YouTube video based on its captions.');

	// Result state
	let loading = $state(false);
	let streaming = $state(false);
	let status = $state('');
	let videoUrl = $state('');
	let videoTitle = $state('');
	let summaryText = $state('');
	let cancelStream: (() => void) | null = null;

	let hasResult = $derived(!!videoUrl);

	onMount(async () => {
		await refreshSummaries();
	});

	async function refreshSummaries() {
		try {
			summaries = await listSummaries();
		} catch {}
	}

	function handleNewChat() {
		activeSummaryId = null;
		url = '';
		videoUrl = '';
		videoTitle = '';
		summaryText = '';
		status = '';
		loading = false;
		streaming = false;
		if (cancelStream) {
			cancelStream();
			cancelStream = null;
		}
	}

	function handleSelectSummary(summary: Summary) {
		if (cancelStream) {
			cancelStream();
			cancelStream = null;
		}
		activeSummaryId = summary.id;
		videoUrl = summary.video_url;
		videoTitle = summary.video_title;
		summaryText = summary.summary_text;
		url = summary.video_url;
		model = summary.model;
		language = summary.language;
		prompt = summary.prompt;
		status = '';
		loading = false;
		streaming = false;
	}

	async function handleDeleteSummary(id: string) {
		try {
			await deleteSummary(id);
			summaries = summaries.filter(s => s.id !== id);
			if (activeSummaryId === id) {
				handleNewChat();
			}
		} catch {}
	}

	function handleSubmit() {
		if (!url.trim() || loading) return;

		loading = true;
		streaming = false;
		status = '';
		summaryText = '';
		videoUrl = url;
		videoTitle = '';
		activeSummaryId = null;

		cancelStream = streamSummarize(
			{ url, model, language, prompt },
			{
				onStatus(msg) {
					status = msg;
				},
				onMeta(meta) {
					activeSummaryId = meta.id;
					videoTitle = meta.video_title;
				},
				onToken(token) {
					if (!streaming) streaming = true;
					summaryText += token;
				},
				onError(msg) {
					status = `Error: ${msg}`;
					loading = false;
					streaming = false;
				},
				async onDone() {
					loading = false;
					streaming = false;
					cancelStream = null;
					await refreshSummaries();
				}
			}
		);
	}
</script>

<div class="app-layout">
	<Sidebar
		{summaries}
		{activeSummaryId}
		collapsed={sidebarCollapsed}
		onSelect={handleSelectSummary}
		onDelete={handleDeleteSummary}
		onNewChat={handleNewChat}
		onToggle={() => sidebarCollapsed = !sidebarCollapsed}
	/>

	<main class="main-content">
		<div class="content-wrapper" class:centered={!hasResult}>
			<div class="brand" class:hidden={hasResult}>
				<h1>Ninefingers</h1>
				<p>Summarize any YouTube video</p>
			</div>

			<InputArea
				bind:url
				bind:model
				bind:language
				bind:prompt
				{loading}
				lifted={hasResult}
				onSubmit={handleSubmit}
			/>

			<SummaryView
				{videoUrl}
				{videoTitle}
				{summaryText}
				{status}
				{streaming}
			/>
		</div>
	</main>
</div>

<style>
	.app-layout {
		display: flex;
		height: 100vh;
		overflow: hidden;
	}

	.main-content {
		flex: 1;
		overflow-y: auto;
		padding: 24px;
	}

	.content-wrapper {
		display: flex;
		flex-direction: column;
		min-height: 100%;
		transition: justify-content 0.4s ease;
	}

	.content-wrapper.centered {
		justify-content: center;
	}

	.brand {
		text-align: center;
		margin-bottom: 32px;
		transition: opacity 0.3s ease, max-height 0.3s ease;
		max-height: 200px;
		overflow: hidden;
	}

	.brand.hidden {
		opacity: 0;
		max-height: 0;
		margin-bottom: 0;
	}

	.brand h1 {
		font-size: 2.2rem;
		font-weight: 600;
		color: var(--ash-grey);
		margin-bottom: 6px;
	}

	.brand p {
		color: var(--text-muted);
		font-size: 1rem;
	}
</style>
