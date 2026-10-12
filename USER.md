# USER.md — who you're building for

## Human

- Intermediate backend dev, beginner AI user. Explain plainly, no lectures.
- Not a frontend dev — assemble UI from proven libs (server shells + CDN board widget), no bespoke design system. Game Review interactivity (drag, arrows, graph) is wanted, but via libraries, not hand-built.
- GUI taste: clean and modern (chessigma-style report layout, wooden board) — explicitly NOT lichess-style density. Current UI is a first pass; visual polish is a later sprint, backend first.
- Token-conscious: confirm briefly before big writes; short summaries after, not essays.

## Stack & conventions (defaults unless told otherwise)

- GOAT monolith: Go + Gin + templ + htmx + Alpine.js + Tailwind, single repo.
- GORM with `gorm.Model` — personal convention, always use it for models.
- SQLite locally now; manual Dialector swap to Postgres later. No repo-interface over-abstraction; owner edits code directly.
- Raw UCI subprocess for Stockfish — no wrappers (they hide knobs that matter).
- Board interactivity: pinned CDN ESM only — `cm-chessboard@8.6.0` + `chess.js@1.4.0` (ADR-8). FEN never user-visible (ADR-9).
- Stdlib + small deps first; simple backend logic over clever caching.

## Workflow

- Confirm plan / output shape before writing files.
- Update `PROGRESS.md` after every task; `DECISIONS.md` only for real architectural trade-offs.
- Verify with `go build`, `go vet`, `go test` before reporting done.
