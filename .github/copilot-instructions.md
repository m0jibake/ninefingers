# Copilot Instructions

## Project Overview

Ninefingers is a Go CLI + web UI that summarizes YouTube videos by fetching captions with `yt-dlp` and streaming them through NVIDIA's LLM API.

## Build & Run

```sh
# CLI usage
go build -o ninefingers .
./ninefingers "<youtube-url>" --model "moonshotai/kimi-k2-instruct" -v

# Web UI (production)
make build
./ninefingers serve --port 8080

# Development (Go API + SvelteKit HMR)
make dev
```

## Prerequisites

- Go 1.25+
- Node.js 22+ (for the SvelteKit frontend)
- `yt-dlp` must be installed (`pip install yt-dlp`)
- A `.env` file at the project root with `NVIDIA_API_KEY` set (falls back to env vars)

## Architecture

### Backend (Go)

- **`main.go`** — Entry point, calls `cmd.Execute()`.
- **`cmd/root.go`** — Cobra root command (CLI mode) with flags: `--model`, `--language`, `--prompt`, `--verbose`.
- **`cmd/summarize.go`** — Thin CLI wrapper that calls into shared logic.
- **`cmd/serve.go`** — `ninefingers serve` subcommand; starts the HTTP server for the web UI.
- **`internal/summarize/`** — Shared core logic: caption fetching via `yt-dlp`, VTT parsing, LLM streaming with callback-based token delivery.
- **`internal/store/`** — SQLite persistence for summary history (pure-Go via `modernc.org/sqlite`), stored in `~/.ninefingers/ninefingers.db`.
- **`internal/server/`** — HTTP server with SSE streaming endpoint (`POST /api/summarize`) and REST CRUD for history.

### Frontend (SvelteKit)

- **`web/`** — SvelteKit SPA with static adapter. Built output goes to `web/build/`, served by the Go binary.
- **`web/src/lib/api.ts`** — API client with SSE stream parsing for token-by-token display.
- **`web/src/lib/components/`** — `Sidebar.svelte` (history), `InputArea.svelte` (URL + options), `SummaryView.svelte` (embedded video + markdown rendering).
- During development, Vite proxies `/api` requests to the Go server on `:8080`.

## Conventions

- Module path is `ninefingers` (not a full GitHub import path).
- LLM types are defined inline in `internal/summarize/`, not in a separate models package.
- External process calls use `os/exec` directly.
- User-facing CLI output uses emoji prefixes (📥, ✅, 🤖).
- Frontend uses Svelte 5 runes mode (`$state`, `$derived`, `$props`).
- SSE events use named events: `status`, `meta`, `token`, `error`, `done`.
- Color palette: `--ash-grey: #cad2c5`, `--muted-teal: #84a98c`, `--deep-teal: #52796f`, `--dark-slate-grey: #354f52`, `--charcoal-blue: #2f3e46`.
