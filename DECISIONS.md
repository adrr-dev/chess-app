# DECISIONS (ADRs)

## ADR-1: React -> GOAT monolith (Go + Gin + templ + htmx + Alpine + Tailwind)

- Context: solo backend dev, no frontend experience, localhost MVP.
- Decision: single-repo Go monolith, server-rendered templ + htmx partials.
- Why: one binary, no NPM build, no CORS, matches existing GOAT usage.
- Trade-off: complex board interactions (drag, arrows, live variations) harder than React + react-chessboard. Accepted: MVP is import -> analyze -> click-through, explore via SAN input + best-move button.

## ADR-2: SQLite now, manual Postgres swap later

- Context: single local user, history = imports only, no auth.
- Decision: GORM + `gorm.Model`, SQLite file `data/chess.db`, AutoMigrate. Later change Dialector to Postgres by hand.
- Why: zero ops for MVP, owner comfortable editing code. No repo-interface abstraction.
- Trade-off: no versioned migrations until Postgres cutover.

## ADR-3: Raw UCI subprocess, no wrappers, no Python

- Context: need full Stockfish control (depth, PV, mate scores).
- Decision: Go spawns `stockfish` binary, speaks UCI over stdin/stdout (`uci`, `isready`, `position`, `go depth N`, parse `info`/`bestmove`).
- Why: wrappers hide PV/score details; Python service adds a second runtime for no gain.
- Trade-off: must implement UCI parsing + proc lifecycle (stop/kill on cancel).

## ADR-4: Transient analysis, persist `games` only

- Context: want simple backend, no re-import friction.
- Decision: persist `Game` (source, pgn_text, ordered moves JSON, player_color, result). Never persist evals/best lines/`game_moves`. Every Analyze runs from scratch.
- Why: no cache invalidation, no stale-depth logic.
- Trade-off: repeat Analyze pays full Stockfish time.

## ADR-5: Single-flight analysis (`409 Busy`)

- Context: localhost single user, one engine proc.
- Decision: global mutex, one run at a time. Second start -> `409`.
- Why: avoids concurrent UCI procs contending for CPU and complicating progress state.
- Trade-off: no queue; user must wait/retry.

## ADR-6: CSV-only MVP, `.xlsx` deferred

- Context: custom Excel format `white,black` proposed.
- Decision: accept `.csv` with header `white,black`. Native `.xlsx` parse deferred; tell user Save-As CSV.
- Why: `encoding/csv` stdlib, no extra dep for MVP.

## ADR-7: Poll over SSE for analyze progress (MVP)

- Context: Stockfish runs seconds-minutes, htmx POST can't block.
- Decision: `POST analyze` -> redirect, `GET status` polled every ~800ms; done state reloads the page to load review JSON into the widget.
- Why: simplest with htmx (`hx-trigger="every 800ms"`), no streaming infra.
- Trade-off: full reload after analysis; SSE upgrade path kept open.

## ADR-8: Interactive board via pinned CDN ESM (cm-chessboard + chess.js)

- Context: server-rendered Unicode board can't drag; user expects chess.com game review (pieces move, arrows).
- Decision: `cm-chessboard@8.6.0` (drag/click input, arrows) + `chess.js@1.4.0` (client legality/FEN), pinned versions via jsdelivr/unpkg, no npm build. No React.
- Why: dependency-free, SVG, proven markers/arrows API; keeps Go monolith intact.
- Trade-off: browser needs network for CDN; version bumps are manual.

## ADR-9: FEN never user-visible

- Context: raw FEN in a textbox read as "encryption" — dev shortcut leaked into UI.
- Decision: FENs live in embedded JSON + JS memory only. No inputs, text nodes, or URL params show FEN.
- Trade-off: noscript users lose board stepping (move list still server-rendered).

## ADR-10: Hand-rolled SVG eval graph, no chart lib

- Context: need win-% curve with click-to-jump + critical dots.
- Decision: inline SVG polyline computed client-side from per-ply cp/mate. No chart dependency.
- Why: ~20 lines, zero dep weight for a simple curve.

## ADR-11: Server-generated coach explanations

- Context: review needs "why" text without an LLM.
- Decision: `analyze.Explain` templates sentences from class + eval delta + best move; client only displays/steps.
- Why: deterministic, testable in Go, no inference cost.
- Trade-off: phrasing is fixed; LLM color deferred post-MVP.
