# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GitReviewAI is an AI-powered GitLab Merge Request code review bot. It receives GitLab webhook events, fetches MR diffs, sends them to an OpenAI-compatible LLM with tool-calling support, and posts review comments (line-level and summary) back to the MR. It includes a web frontend for managing reviews.

## Build and Run

```bash
# Full build (frontend + Go binary)
make build

# Build frontend only
make build-frontend

# Build Go binary only (requires frontend already built)
make build-go

# Run
# First, copy config.yaml.example to config.yaml and fill in your values
cp config.yaml.example config.yaml
go run cmd/server/main.go
go run cmd/server/main.go -config /path/to/config.yaml

# Development mode (run frontend and backend separately)
make dev-frontend  # Vite dev server on :5173
make dev-go        # Go backend on :8080

# Docker build (static binary, CGO disabled)
docker build -t gitreviewai .

# Clean build artifacts
make clean
```

## Architecture

The service is a single Go binary that serves both the API and embedded Vue frontend.

### Source Files

**Backend (`internal/`):**

1. **`internal/webhook/handler.go`** — HTTP handler at `/webhook`. Validates `X-Gitlab-Token`, parses the GitLab MR event JSON, filters for `open`/`update` actions, then dispatches `ReviewMR()` in a goroutine (returns 200 immediately).

2. **`internal/reviewer/reviewer.go`** — Core orchestrator. Fetches MR info and diffs via the GitLab client, filters/classifies changed files, runs the AI tool-calling loop, persists results to SQLite, and optionally auto-submits to GitLab based on settings.

3. **`internal/ai/client.go`** — Wraps the OpenAI Go SDK (`openai-go`). Runs a multi-turn chat loop where the AI calls tools to gather context or produce output. Compresses conversation history (at >25 messages).

4. **`internal/ai/tools.go`** — Defines the 8 tools available to the AI: `FinishReview`, `GetMoreChanges`, `AddLineComment`, `AddReviewComment`, `GenerateMDReport`, `ReadFile`, `FindInFile`, `GetURL`. Also contains the full system prompt (in Chinese).

5. **`internal/gitlab/client.go`** — REST client for GitLab API v4. Uses `PRIVATE-TOKEN` auth. Supports: get MR info, get MR changes, post notes (returns note ID), create/publish draft notes, read file content.

6. **`internal/config/config.go`** — Loads `config.yaml` into a struct. All config is file-based. See `config.yaml` for the full commented reference.

7. **`internal/database/`** — SQLite database layer:
   - `db.go` — Opens database, runs migrations
   - `mr_store.go` — Merge request CRUD (Upsert, GetByID, List with pagination)
   - `comment_store.go` — Comment CRUD (Create, CreateBatch, ListByMRID, UpdateStatus)
   - `report_store.go` — Report CRUD (Create, ListByMRID, UpdateStatus)
   - `setting_store.go` — Settings CRUD (Get, Set, GetAutoSubmit)

8. **`internal/auth/auth.go`** — JWT authentication. LoginHandler validates password, returns JWT. GinMiddleware validates JWT from Authorization header or cookie.

9. **`internal/api/`** — API handlers and embedded frontend:
   - `handler.go` — Route registration, serves embedded Vue SPA
   - `mr_handlers.go` — MR list, detail, comment/report submission endpoints
   - `frontend/` — Vue 3 frontend (embedded via `go:embed`)

10. **`cmd/server/main.go`** — Entrypoint. Loads config, validates required fields, opens database, creates reviewer with DB access, sets up gin router with auth middleware, registers routes, runs with graceful shutdown.

**Frontend (`internal/api/frontend/`):**

- Vue 3 + Vite + Naive UI
- Pages: Login, MR List, MR Detail, Settings
- Markdown rendering with markdown-it + highlight.js

### Request Flow

```
GitLab Webhook POST /webhook
  → webhook.Handler.HandleWebhook()
    → validates X-Gitlab-Token
    → parses WebhookEvent JSON
    → filters for merge_request + open/update
    → spawns goroutine:
       → reviewer.ReviewMR(projectID, mrIID)
          → gitlab.Client.GetMRInfo() → MRInfo
          → database.MRStore.Upsert() → persist MR to SQLite
          → gitlab.Client.GetMRChanges() → []MRChange + DiffRefs
          → filterChanges() → filtered []MRChange
          → ai.Client.ChatWithLimit() → multi-turn AI loop:
             → AI calls tools → handleToolCall() dispatches
             → if >25 messages, compressMessages()
          → persist comments/reports to SQLite
          → if auto_submit enabled: submit to GitLab + record note IDs
```

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/login` | No | Password login, returns JWT |
| GET | `/api/mrs` | Yes | List MRs with pagination |
| GET | `/api/mrs/{id}` | Yes | MR detail with comments & reports |
| POST | `/api/comments/{id}/submit` | Yes | Submit single comment to GitLab |
| POST | `/api/reports/{id}/submit` | Yes | Submit single report to GitLab |
| POST | `/api/mrs/{id}/submit-all` | Yes | Submit all pending for an MR |
| GET | `/api/settings` | Yes | Get settings |
| PUT | `/api/settings` | Yes | Update settings |
| POST | `/webhook` | No | GitLab webhook |
| GET | `/health` | No | Health check |

## Configuration

Configuration is in `config.yaml` (YAML), which is gitignored. Copy `config.yaml.example` to `config.yaml` and fill in your values. Required fields:
- `gitlab_token` — GitLab Personal Access Token
- `openai_api_key` — OpenAI API key
- `password` — Login password for web UI
- `jwt_secret` — JWT signing secret (min 32 chars)

Optional: `openai_base_url`, `port`, `webhook_token`, `max_line_comments`, `ignore_paths`, `log_level`, `jwt_expiry`, `db_path`. See `config.yaml.example` for all options and defaults.

## Database

SQLite database (default: `./data/gitreviewai.db`). Tables:
- `merge_requests` — UNIQUE(project_id, mr_iid), review_status tracks state
- `comments` — line/review comments with gitlab_note_id for tracking
- `reports` — AI-generated reports with gitlab_note_id
- `settings` — key-value store for runtime config (auto_submit)

## Language Notes

- Documentation (README, system prompt, requirements doc) is primarily in Chinese.
- The system prompt in `internal/ai/tools.go` governs all AI review behavior.
