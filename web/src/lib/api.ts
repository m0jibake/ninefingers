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
			let currentEvent = '';

			while (true) {
				const { done, value } = await reader.read();
				if (done) break;

				buffer += decoder.decode(value, { stream: true });
				const lines = buffer.split('\n');
				buffer = lines.pop() || '';

				for (const line of lines) {
					if (line.startsWith('event: ')) {
						currentEvent = line.slice(7).trim();
					} else if (line.startsWith('data: ')) {
						const data = line.slice(6);
						if (currentEvent) {
							handleEvent(currentEvent, data, callbacks);
							currentEvent = '';
						}
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
