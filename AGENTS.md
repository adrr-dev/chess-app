# AGENTS.md

## Project

Free localhost chess Game Review (chess.com-style). Single-repo Go monolith (GOAT).
No auth, single user, Stockfish runs locally.

## Docs (read before coding)

- `PRD.md` — Game Review scope, acceptance.
- `ARCHITECTURE.md` — layout, routes incl. `review.json`, concurrency.
- `DESIGN.md` — widget spec, review.json shape, explanations, parsers, thresholds.
- `USER.md` — human prefs and conventions.

## Commands

```bash
templ generate
go run ./cmd/server
go test ./...
```

Env: `PORT=8080` `STOCKFISH_PATH=stockfish` `DB_PATH=data/chess.db` `ANALYSIS_DEPTH=18`

Requires local Stockfish binary on PATH. Browser needs network for CDN libs.

## Stack rules

- Go + Gin + GORM (`gorm.Model`, SQLite now, manual Dialector swap to Postgres later).
- templ shells + htmx partials + Alpine.js + Tailwind. No React, no npm build.
- Board interactivity: pinned CDN ESM only — `cm-chessboard@8.6.0` + `chess.js@1.4.0`. Do not add other frontend deps without an ADR.
- Backend owns all DB access. SQLite persists `games` only (moveset + player color). No `game_moves` table.
- **FEN strings are never rendered as user-visible text** (no inputs, no text nodes, no URL params). They live in embedded JSON / JS memory only.

## Engine rules

- Raw UCI subprocess only. No wrappers, no Python.
- Default `go depth 18`, Threads 1.
- Single-flight analysis: second `POST analyze` while busy -> `409 Busy`. Explore shares the proc (serialized); explore during a run -> `409`.
- Analysis is transient, re-run from scratch every time. Never persist evals/best lines.

## Workflow

Build order: `chessparse` -> `engine` UCI -> `analyze` classify+explain -> routes + `review.json` -> widget (`review.js`) -> polish.
Update `PROGRESS.md` after every task; `DECISIONS.md` only for real architectural trade-offs.
Verify with `go build`, `go vet`, `go test` + curl smoke before reporting done.
