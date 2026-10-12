# PRD — Chess Analysis Helper (Game Review)

## 1. Overview

- **What:** Free, local-first chess **Game Review** in the style of chess.com review: import a finished game, get accuracy + eval graph + a coach walkthrough of key moments, on an interactive board.
- **Who:** Singleplayer chess players reviewing their own completed games.
- **Why:** Free alternative, runs on localhost, no account required for MVP.
- **Where:** Web app, localhost only for now. Single-repo Go monolith (GOAT: Go + Gin + templ + htmx + Alpine.js + Tailwind) with CDN board libs (no npm build).

## 2. Goals / Non-Goals

### Goals (MVP)

- Import a completed game via PGN text/file or CSV (`white,black`).
- Validate and parse the game, store the moveset locally (history = imports only).
- Ask `I played as: White | Black` after import.
- **Interactive board**: drag/click pieces to move, step through the game, flip orientation, arrows for best/played moves. Pieces must actually move.
- **Review panel**: overall accuracy (you/white/black), eval/win-% graph across the game, class counts.
- **Coach walkthrough**: step through critical moments with generated explanations ("Missed win — Qxf7 mated in 1"), best line drawn as an arrow.
- **Explore by moving**: from any position, drag an alternate move; engine evals the new line on demand and shows the verdict. No typing SAN, no visible FEN ever.
- On `Analyze`, run Stockfish from scratch and fill the review.

### Non-Goals (MVP)

- No live play from scratch, no play-vs-engine games.
- No auth / accounts.
- No Postgres / Supabase in MVP (SQLite only).
- No Chess960 / Fischer Random support.
- No LLM natural-language commentary (template explanations from eval deltas only).
- No persisted analysis cache (re-analyze from scratch every time).

Post-MVP: accounts, Postgres/Supabase deploy, multi-user, LLM commentary, 960, variations persistence.

## 3. Users & UX Flow

- **User:** Single local user, no login.
- **Flow:**
  1. Land on import page (history below).
  2. Import PGN or CSV, pick `I played as: [White|Black]`.
  3. Game page: interactive board (final position) + empty review panel + Analyze button.
  4. Press `Analyze` -> progress -> review fills in: accuracy header, eval graph, move list with verdicts.
  5. Browse: drag the (disabled) main line? No — step with Prev/Next, click moves/graph, board animates, arrows show best vs played.
  6. Review mode: "Start review" walks critical moments in order with coach text.
  7. Explore: toggle "Try moves", drag any legal move on the board -> instant engine verdict + arrow. Never persisted, never a FEN textbox.

## 4. Functional Requirements

### 4.1 PGN import (unchanged)

- Accept `.pgn` file upload and paste-text; headers may share one line.
- Must parse standard headers + movetext in SAN; reject illegal moves with move number + SAN.
- Store raw `pgn_text` verbatim plus normalized ordered `moves` list.

### 4.2 CSV import (unchanged)

- Header exactly `white,black`; rows `e4,e5`; missing black allowed only on last row.
- Player color from post-import selector, never from the file.

### 4.3 Player color (unchanged)

- `player_color: white | black | unknown`; drives board orientation + your-accuracy.

### 4.4 Analysis + revieweli (transient, no cache)

- Engine: Stockfish via raw UCI, fixed depth (default 18, env override). Every Analyze from scratch; single-flight (`409 Busy`).
- Per ply (in-memory): eval, mate, best move (SAN + UCI), cp-loss, class, **coach explanation string**, post-move FEN.
- `GET review.json`: full review payload for the board widget (plies + evals + explanations + accuracies + critical list).
- Classification set: `Best | Good | Inaccuracy | Mistake | Blunder | Missed Win` (thresholds in DESIGN.md).

### 4.5 History (imports only)

- List/open/delete past imports. Opening loads the moveset; analysis is re-run.

## 5. Data (summary; full schema in DESIGN.md)

- `Game` (only persisted table, IS the history): `Source (pgn|csv)`, `PgnText`, `MovesJson` (single ordered SAN array), `PlayerColor`, `Result`. GORM `gorm.Model`.
- No persisted `game_moves`. Per-move analysis is transient per run.
- `users` table: post-MVP only.

## 6. Tech Constraints

- Single-repo Go monolith: Gin + GORM + templ + htmx + Alpine + Tailwind.
- Board interactivity via pinned CDN ESM libs (no npm): `cm-chessboard@8` (drag/click/arrows) + `chess.js@1` (client move legality). No React.
- Backend owns DB + engine. Frontend sends moves/FENs to eval endpoints; **FEN strings are never rendered as user-visible text**.
- MVP DB: local SQLite, manual Postgres swap later.
- Engine: raw UCI subprocess to Stockfish. `notnil/chess` server-side for replay/FEN.

## 7. MVP Acceptance

- [ ] Import sample chess.com PGN -> Analyze -> accuracy header + eval graph + verdict move list all visible.
- [ ] Board pieces drag; stepping moves animates; flip works; best-line arrows show on critical moments.
- [ ] Review mode walks critical moments with sensible coach text (spot-check Scholar's mate: "Missed win"/mate callout).
- [ ] Explore: drag alternate move -> verdict appears, no typing, no FEN visible anywhere.
- [ ] Second Analyze while running -> busy message, no engine clash.
- [ ] Reload -> history persists, analysis re-runnable; illegal PGN/CSV errors are actionable.
- [ ] No paid APIs; everything local except CDN libs.

## 8. Open Questions (resolve in DESIGN.md)

- Stockfish depth/time default (18 vs faster localhost default).
- Exact cp-loss thresholds per class (locked v1, tune later).
- CSV-only vs native `.xlsx` (still deferred).
