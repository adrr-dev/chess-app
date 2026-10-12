# PROGRESS

## Sprint: Game Review rebuild (interactive board + coach walkthrough)

- [x] Docs re-scoped: PRD/ARCHITECTURE/DESIGN/AGENTS -> Game Review; ADR-8..11; USER.md frontend line
- [x] Backend (unchanged, still green): chessparse, engine, SQLite, import/history/delete
- [x] `analyze`: PlyResult += Before/Fen/BestUCI + `Explain` + tests (delivered-mate fix: post-mate mate-0 reads as "no mate", now overridden via chess-lib outcome)
- [x] Handler `review.json` + FEN hidden (explore partial uses data attr; JSON for widget)
- [x] `review.js` widget + game shell rewrite; server board table + `/position` dropped; `internal/board` deleted
- [x] Smoke: Scholar's mate review correct; widget shell verified (valid review-data JSON, no FEN textbox, static JS 200, JS syntax OK); busy path still 409
- [x] Fixes: templ ignores exprs inside `<script>` -> embed via `templ.JSONScript` (board now renders); `critical: null` -> `[]` both sides (clean games crashed review mode); delivered-mate override; auto-reload scoped to poll partial only (full-page renders no longer loop-reload)
- [x] Review fixes: flip removed (orientation = your color); bar updates first + arrows in try/catch; Prev/Next step criticals in review mode; coach notes on criticals only; explore returns evalCp/mateIn and drives the bar; DESIGN §1 follows chessigma layout, wooden palette

- [x] Docs: PRD, ARCHITECTURE, DESIGN, AGENTS
- [x] Docs: DECISIONS (ADRs), PROGRESS (this file), Makefile
- [x] Scaffold: `go mod init`, dirs (`cmd/server`, `internal/...`, `web/...`, `data/`) — `go build`, `go vet`, skeleton `GET /` + `/health`, SQLite AutoMigrate verified
- [x] Env: Go 1.27.1, gcc, templ v0.3.1070 (`/home/art/go/bin/templ` — add to PATH)
- [x] Env: stockfish binary — installed via AUR, Stockfish 19 at `/usr/bin/stockfish`, UCI handshake verified
- [x] `chessparse`: PGN + CSV -> []SAN + tests (8 tests pass; result marker fix, Qh4 legality lesson: e-pawn moved so d8-h4 is open)
- [x] `engine`: raw UCI client + tests (4 tests pass vs Stockfish 19: startpos, mate-in-1, sign conversion, bad FEN)
- [x] `analyze`: single-flight + classification + tests (classify table 15 cases, accuracy, uciToSAN, service run/busy/illegal; TryLock->running-flag fix: mutex is free during runs so Progress stays readable)
- [x] Routes + GORM SQLite + templ shells (import PGN/CSV, history, game page, wooden board, move list, explore; `go build`/`vet` clean)
- [x] Analyze wiring: `POST analyze` 202/409, htmx `GET status` poll, `GET position?ply=n` partial, `POST explore` — smoke-tested E2E at depth 6
- [ ] Poll UI polish (current-move highlight in list, next-mistake jump) — polish, not MVP-blocker

## Active

On break —MVP review loop complete E2E. Docs refreshed (README setup guide, ARCHITECTURE as-built, .gitignore for push).

## Next (later)

1. Browser trial at depth 18 with a real chess.com PGN.
2. GUI polish sprint: clean modern review look (chessigma layout, wooden palette), not lichess-style. Current UI is functional first pass.
3. Features: ECO opening book, Brilliant detection, promotion picker in explore, offline CDN fallback.
4. `DECISIONS.md`: ADR-8..11 added for this rebuild; delivered-mate override is bug-level (covered by ADR-3).
