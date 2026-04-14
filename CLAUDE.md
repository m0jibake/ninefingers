# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```sh
# Production build (frontend then Go binary)
make build          # runs build.sh: npm run build in web/, then go build -o ninefingers .

# Development (live reload — Go API on :8080, SvelteKit on :5173)
make dev

# Run the server
./ninefingers serve

# Run CLI summarization
./ninefingers "https://www.youtube.com/watch?v=..." --model "moonshotai/kimi-k2-instruct" -v

# Clean build artifacts
make clean

# Go tests (none yet, but standard)
go test ./...
```

## Environment

Create `.env` with:
- `NVIDIA_API_KEY` — required for all summarization
- `GITHUB_TOKEN` + `BLOG_REPO` (format: `owner/repo`) — optional, for blog export feature

Requires `yt-dlp` on PATH (`pip install yt-dlp`) and Node.js 22+ (uses `~/.nvm/nvm.sh` if available).

## Architecture

The app has two modes: **CLI** (direct stdout streaming) and **web server** (SSE streaming to browser).

### Go backend (`/`)

- `main.go` → `cmd/` (Cobra CLI)
- `cmd/root.go` — root command (CLI summarize) and `serve` subcommand
- `internal/summarize/summarize.go` — caption fetching via `yt-dlp` (downloads `.vtt`, strips tags, deduplicates lines) and LLM streaming via NVIDIA NIM API (`https://integrate.api.nvidia.com/v1/chat/completions`) with OpenAI-compatible SSE format
- `internal/store/store.go` — SQLite persistence via `modernc.org/sqlite` (no CGo); DB stored at `~/.ninefingers/ninefingers.db`; schema auto-migrated on startup
- `internal/server/server.go` — HTTP server using stdlib `net/http` with routes: `POST /api/summarize` (SSE stream), `GET/DELETE /api/summaries[/{id}]`, `POST /api/export-to-blog` (GitHub API)

### SSE protocol (`POST /api/summarize`)

Events sent in order: `status` → `meta` (JSON with `id`, `video_title`) → `token` (repeated) → `done` (or `error`). The summary record is saved before streaming starts, then updated with full text after streaming completes.

### SvelteKit frontend (`web/`)

- `web/src/lib/api.ts` — typed API client; `streamSummarize()` manually parses SSE (not `EventSource`) to support `POST` requests
- `web/src/lib/components/SummaryView.svelte` — main summary display component
- In dev mode, SvelteKit proxies API calls to the Go server on `:8080`
- In production, the built frontend (`web/build/`) is embedded in the Go binary and served as static files via `SetStaticHandler`

### Default model

`z-ai/glm4.7` — used when no `--model` flag is provided. Other supported: `moonshotai/kimi-k2-instruct`, Llama, DeepSeek, Gemma variants (all via NVIDIA NIM).
