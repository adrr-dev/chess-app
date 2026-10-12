# ARCHITECTURE — chess-app (as implemented)

Single-repo Go monolith (GOAT): Go + Gin + GORM + templ + htmx + Alpine.js +
Tailwind CDN, plus pinned CDN ESM board libs. Localhost, single user, no auth.

## 1. Stack

- Backend: Go, Gin (routes), GORM (`gorm.Model`, SQLite `data/chess.db`, AutoMigrate).
- Views: templ shells (`web/templates/*.templ` → generated `*_templ.go`, gitignored, rebuilt by `make run`); htmx for analyze-progress polling; Tailwind via CDN.
- Board widget: `web/static/review.js` (plain ESM) + `cm-chessboard@8.6.0` (drag/click, arrows) + `chess.js@1.4.0` (client legality/FEN). No npm, no React.
- Engine: Stockfish sidecar, raw UCI subprocess (`uci/isready/position/go depth N`), Threads 1. Server move logic: `notnil/chess`.

## 2. File structure (actual)

```text
cmd/server/main.go        # env (PORT/DB_PATH/STOCKFISH_PATH/ANALYSIS_DEPTH), /static, Register
internal/models/game.go   # Game: gorm.Model + Source/PgnText/MovesJSON/PlayerColor/Result
internal/db/db.go         # Open(path): mkdirall + sqlite open + AutoMigrate
internal/chessparse/      # ParsePGN (tag/comment/NAG tolerant, result kept) + ParseCSV (strict white,black)
internal/engine/client.go # New/Close/NewGame/EvalFEN; side-to-move -> White-relative; ctx cancel sends stop
internal/analyze/         # ClassifyPlayed/CpLossOf/Accuracy/Explain/uciToSAN + Service (single-flight) + replay
internal/handler/
  app.go                  # App{db, analyzer, eval, depth, runs}, lockedEval, helpers (loadGame, fensOf via games.go, evalText/Pct, moveRows)
  games.go                # Index/CreateGame/ShowGame/DeleteGame
  analyze.go              # StartAnalyze/AnalyzeStatus/Explore + statusOf/classOrder
  review.go               # ReviewJSON + plyJSON/reviewPayload builders
web/templates/            # layout/index/game/partials (.templ), view.go (GameRow/MoveRow/ReviewBoot/...)
web/static/review.js      # widget: stepping, SVG graph, review walk, explore drags
```

`internal/board` was built then deleted (server no longer renders squares; widget owns the board).

## 3. API endpoints (actual)

| Method | Route | Req | Resp |
|---|---|---|---|
| GET | `/` | — | HTML import form + history table |
| GET | `/health` | — | `ok` |
| POST | `/games` | multipart `file` (ext `.csv`→CSV else PGN) or `pgn_text`/`csv_text` + `player_color` | `303` to `/games/:id`; `400` with move/row detail |
| GET | `/games/:id` | `?ply=n` optional (initial widget ply) | HTML shell + `review-data` JSON (`gameID/ply/orientation/fens/moves/review/exploreURL`) |
| DELETE / POST | `/games/:id`, `/games/:id/delete` | — | delete row + drop run; `303 /` |
| POST | `/games/:id/analyze` | — | `303` back to game; `409` when engine busy |
| GET | `/games/:id/analyze/status` | — (htmx poll ~800ms) | HTML partial: none/running(+rows)/done(+summary+one-shot reload)/error |
| GET | `/games/:id/review.json` | — | full transient review or `404` (never analyzed) / `409` (running) / `500` (run error) |
| POST | `/games/:id/explore` | form `fen` + `san` | HTML partial, or JSON `{fen,line,best,evalCp,mateIn}` with `Accept: application/json`; `400` bad fen/illegal; `409` when a run holds the engine |
| GET | `/static/*` | — | `web/static` files (`review.js`) |

## 4. Schema (actual)

```sql
games(id, created_at, updated_at, deleted_at, source, pgn_text, moves_json, player_color, result)
```

`moves_json`: single ordered SAN array. No `game_moves`, no `users` (post-MVP).
Transient per-run state (memory only, `runs map[uint]*runState`): results, summary, error, finished.

## 5. review.json shape (actual)

```json
{"plies":[{"ply":1,"san":"e4","fen":"...","evalCp":25,"mateIn":0,"best":"e5",
"bestUci":"e7e5","cpLoss":5,"class":"Best","explain":"Best move."}],
"whiteAcc":82.1,"blackAcc":74.0,"yourAcc":82.1,
"counts":[{"class":"Best","n":12}, ...],"critical":[14,23]}
```

## 6. Concurrency (actual)

- `analyze.Service`: `running` flag under mutex; second `Analyze` → `ErrBusy` → HTTP `409`.
- One UCI proc (`lockedEval` mutex) shared by analysis runs and explore one-shots.
- Analysis goroutine per started run; progress via `Progress()`; results streamed through `onPly` into the run state.

## 7. Known gaps (next sprints, see PROGRESS.md)

- GUI is minimal/first-pass: target is a clean modern review look (chessigma layout, wooden palette), not lichess-style density. Board widget + accuracy/graph/list exist; visual polish pending.
- No opening book (ECO names) yet; no Brilliant detection; promotions auto-queen in explore; CDN requires browser network.
