# chess-app — free local chess Game Review

Import a finished game (PGN from chess.com, or `white,black` CSV), run Stockfish
over every move, and step through an interactive review: accuracy, eval graph,
move verdicts, critical-moment walkthrough with coach notes, best-line arrows,
and drag-to-explore alternate lines. No account, no limits, localhost only.

## Prerequisites

- Go 1.24+ (`go version`)
- Stockfish binary on PATH — Arch/CachyOS: `yay -S stockfish` (AUR). Verify: `which stockfish`
- `templ` codegen for templates: `make tools` (installs to `~/go/bin`; add to PATH — fish: `fish_add_path ~/go/bin`)
- gcc (SQLite driver build)
- Browser with network access (board/piece assets + chess libs load from pinned CDNs; engine itself runs locally)

## Run

```bash
make run          # templ generate, then server on :8080
```

Open http://localhost:8080 — import page + history.

## Environment variables

| Var | Default | Meaning |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `DB_PATH` | `data/chess.db` | SQLite file (created on first run) |
| `STOCKFISH_PATH` | `stockfish` | Engine binary (PATH lookup or absolute path) |
| `ANALYSIS_DEPTH` | `18` | Stockfish fixed depth per position (lower = faster, e.g. `6` for smoke tests) |
| `GIN_MODE` | `debug` | Set `release` to quiet Gin logs |

## Makefile targets

| Target | Does |
|---|---|
| `make run` | `templ generate` (if present) + `go run ./cmd/server` |
| `make test` | `go test ./...` (engine tests need stockfish; others are pure) |
| `make tools` | Installs `templ` codegen |
| `make tailwind-watch` | Needs `tailwindcss` binary; MVP uses the Tailwind CDN instead |
| `make clean` | Deletes `data/chess.db` |

## Import formats

- **PGN**: upload `.pgn` or paste text (chess.com export works; headers may share one line). Illegal moves rejected with move number + SAN.
- **CSV**: header exactly `white,black`, one fullmove per row (`e4,e5`). Missing black cell allowed only on the last row.

After import pick `I played as: White | Black` (drives board orientation + your-accuracy).

## Status codes worth knowing

- `303` after import/analyze/delete (redirects)
- `400` on bad PGN/CSV/FEN with the offending move/row in the message
- `404` game not found / review requested before analyzing
- `409` engine busy (one analysis at a time; explore during a run too)

## Docs

- `PRD.md` — scope + acceptance · `ARCHITECTURE.md` — as-built layout/routes/schema
- `DESIGN.md` — widget spec, review.json, thresholds · `DECISIONS.md` — ADRs
- `AGENTS.md` — AI contributor rules · `USER.md` — owner prefs · `PROGRESS.md` — sprint log
