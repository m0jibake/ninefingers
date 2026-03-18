<script lang="ts">
	interface Props {
		url: string;
		model: string;
		language: string;
		prompt: string;
		loading: boolean;
		lifted: boolean;
		onSubmit: () => void;
	}

	let { url = $bindable(), model = $bindable(), language = $bindable(), prompt = $bindable(), loading, lifted, onSubmit }: Props = $props();

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			onSubmit();
		}
	}
</script>

<div class="input-area" class:lifted>
	<div class="main-input-row">
		<input
			type="text"
			class="url-input"
			placeholder="Paste a YouTube URL..."
			bind:value={url}
			onkeydown={handleKeydown}
			disabled={loading}
		/>
		<button class="send-btn" onclick={onSubmit} disabled={loading || !url.trim()}>
			{#if loading}
				<span class="spinner"></span>
			{:else}
				<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg>
			{/if}
		</button>
	</div>

	<div class="options-row">
		<label class="option">
			<span>Model</span>
			<select bind:value={model} disabled={loading}>
				<option value="z-ai/glm4.7">GLM-4.7 (default)</option>
				<option value="moonshotai/kimi-k2-instruct">Kimi K2</option>
				<option value="meta/llama-3.3-70b-instruct">Llama 3.3 70B</option>
				<option value="deepseek-ai/deepseek-r1">DeepSeek R1</option>
				<option value="google/gemma-3-27b-it">Gemma 3 27B</option>
			</select>
		</label>
		<label class="option">
			<span>Language</span>
			<select bind:value={language} disabled={loading}>
				<option value="en">English</option>
				<option value="es">Spanish</option>
				<option value="fr">French</option>
				<option value="de">German</option>
				<option value="pt">Portuguese</option>
				<option value="ja">Japanese</option>
				<option value="ko">Korean</option>
				<option value="zh">Chinese</option>
			</select>
		</label>
		<label class="option prompt-option">
			<span>Prompt</span>
			<input
				type="text"
				bind:value={prompt}
				placeholder="Custom instruction..."
				disabled={loading}
			/>
		</label>
	</div>
</div>

<style>
	.input-area {
		width: 100%;
		max-width: 720px;
		margin: 0 auto;
		transition: transform 0.4s cubic-bezier(0.4, 0, 0.2, 1);
	}

	.input-area.lifted {
		transform: translateY(0);
	}

	.main-input-row {
		display: flex;
		gap: 8px;
		align-items: center;
	}

	.url-input {
		flex: 1;
		padding: 14px 18px;
		background: var(--bg-input);
		border: 1px solid var(--border);
		border-radius: var(--radius);
		font-size: 1rem;
		outline: none;
		transition: border-color 0.15s;
	}

	.url-input:focus {
		border-color: var(--accent);
	}

	.url-input::placeholder {
		color: var(--text-muted);
	}

	.send-btn {
		width: 48px;
		height: 48px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: var(--accent);
		color: var(--charcoal-blue);
		border-radius: var(--radius);
		transition: background 0.15s, opacity 0.15s;
		flex-shrink: 0;
	}

	.send-btn:hover:not(:disabled) {
		background: var(--deep-teal);
		color: var(--ash-grey);
	}

	.send-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.spinner {
		width: 18px;
		height: 18px;
		border: 2px solid transparent;
		border-top-color: currentColor;
		border-radius: 50%;
		animation: spin 0.6s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}

	.options-row {
		display: flex;
		gap: 12px;
		margin-top: 10px;
		flex-wrap: wrap;
	}

	.option {
		display: flex;
		flex-direction: column;
		gap: 4px;
		font-size: 0.75rem;
		color: var(--text-muted);
	}

	.option select,
	.option input {
		padding: 6px 10px;
		background: var(--bg-input);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		font-size: 0.8rem;
		outline: none;
	}

	.option select:focus,
	.option input:focus {
		border-color: var(--accent);
	}

	.prompt-option {
		flex: 1;
		min-width: 150px;
	}

	.prompt-option input {
		width: 100%;
	}
</style>
