export interface Summary {
	id: string;
	video_url: string;
	video_title: string;
	model: string;
	language: string;
	prompt: string;
	summary_text: string;
	created_at: string;
}

export interface SummarizeRequest {
	url: string;
	model: string;
	language: string;
	prompt: string;
}

export async function listSummaries(): Promise<Summary[]> {
	const res = await fetch('/api/summaries');
	if (!res.ok) throw new Error('Failed to fetch summaries');
	return res.json();
}

export async function getSummary(id: string): Promise<Summary> {
	const res = await fetch(`/api/summaries/${id}`);
	if (!res.ok) throw new Error('Failed to fetch summary');
	return res.json();
}

export async function deleteSummary(id: string): Promise<void> {
	const res = await fetch(`/api/summaries/${id}`, { method: 'DELETE' });
	if (!res.ok) throw new Error('Failed to delete summary');
}

export interface ChatMessage {
	id: string;
	summary_id: string;
	role: 'user' | 'assistant';
	content: string;
	created_at: string;
}

export async function listMessages(summaryId: string): Promise<ChatMessage[]> {
	const res = await fetch(`/api/summaries/${summaryId}/messages`);
	if (!res.ok) throw new Error('Failed to fetch messages');
	return res.json();
}

export function streamChat(
	summaryId: string,
	message: string,
	callbacks: StreamCallbacks
): () => void {
	const controller = new AbortController();

	(async () => {
		try {
			const res = await fetch(`/api/summaries/${summaryId}/chat`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ message }),
				signal: controller.signal
			});

			if (!res.ok) {
				const text = await res.text();
				callbacks.onError?.(text || 'Chat request failed');
				return;
			}

			const reader = res.body!.getReader();
			const decoder = new TextDecoder();
			let buffer = '';

			while (true) {
				const { done, value } = await reader.read();
				if (done) break;

				buffer += decoder.decode(value, { stream: true });
				const messages = buffer.split('\n\n');
				buffer = messages.pop() || '';

				for (const msg of messages) {
					let event = '';
					const dataLines: string[] = [];
					for (const line of msg.split('\n')) {
						if (line.startsWith('event: ')) event = line.slice(7).trim();
						else if (line.startsWith('data: ')) dataLines.push(line.slice(6));
						else if (line.startsWith('data:')) dataLines.push(line.slice(5));
					}
					if (event && dataLines.length > 0) {
						handleEvent(event, dataLines.join('\n'), callbacks);
					}
				}
			}
		} catch (err: any) {
			if (err.name !== 'AbortError') callbacks.onError?.(err.message);
		}
	})();

	return () => controller.abort();
}

export interface ExportResult {
	path: string;
	html_url: string;
	commit_sha: string;
}

export async function exportToBlog(
	summaryText: string,
	videoTitle: string,
	videoURL: string
): Promise<ExportResult> {
	const res = await fetch('/api/export-to-blog', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ summaryText, videoTitle, videoURL })
	});
	if (!res.ok) {
		const text = await res.text();
		throw new Error(text || 'Export failed');
	}
	return res.json();
}

export interface StreamCallbacks {
	onStatus?: (msg: string) => void;
	onMeta?: (meta: { id: string; video_title: string }) => void;
	onToken?: (token: string) => void;
	onError?: (msg: string) => void;
	onDone?: () => void;
}

export function streamSummarize(req: SummarizeRequest, callbacks: StreamCallbacks): () => void {
	const controller = new AbortController();

	(async () => {
		try {
			const res = await fetch('/api/summarize', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(req),
				signal: controller.signal
			});

			if (!res.ok) {
				const text = await res.text();
				callbacks.onError?.(text || 'Request failed');
				return;
			}

			const reader = res.body!.getReader();
			const decoder = new TextDecoder();
			let buffer = '';

			while (true) {
				const { done, value } = await reader.read();
				if (done) break;

				buffer += decoder.decode(value, { stream: true });

				// SSE messages are separated by blank lines (\n\n)
				const messages = buffer.split('\n\n');
				buffer = messages.pop() || '';

				for (const msg of messages) {
					let event = '';
					const dataLines: string[] = [];

					for (const line of msg.split('\n')) {
						if (line.startsWith('event: ')) {
							event = line.slice(7).trim();
						} else if (line.startsWith('data: ')) {
							dataLines.push(line.slice(6));
						} else if (line.startsWith('data:')) {
							dataLines.push(line.slice(5));
						}
					}

					if (event && dataLines.length > 0) {
						const data = dataLines.join('\n');
						handleEvent(event, data, callbacks);
					}
				}
			}
		} catch (err: any) {
			if (err.name !== 'AbortError') {
				callbacks.onError?.(err.message);
			}
		}
	})();

	return () => controller.abort();
}

function handleEvent(event: string, data: string, callbacks: StreamCallbacks) {
	switch (event) {
		case 'status':
			callbacks.onStatus?.(data);
			break;
		case 'meta':
			try { callbacks.onMeta?.(JSON.parse(data)); } catch {}
			break;
		case 'token':
			callbacks.onToken?.(data);
			break;
		case 'error':
			try {
				const parsed = JSON.parse(data);
				callbacks.onError?.(parsed.message || data);
			} catch {
				callbacks.onError?.(data);
			}
			break;
		case 'done':
			callbacks.onDone?.();
			break;
	}
}
