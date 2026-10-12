# DESIGN — Chess Analysis Helper (Game Review)

Backend-first, but the review UI is specified here since it drives endpoint shapes.

## 1. Review UI (`web/static/review.js` + game shell)

Layout follows the chessigma game-report pattern: dark page, left column board
with eval bar attached, right column accuracy header + analyze control +
verdict move list; eval graph under the board; coach line beneath it.
Palette is wooden, not chessigma-dark: board light `#EBD2B1` / dark `#A5714A`
(CSS vars `--bl`/`--bd`), page stays dark stone for contrast. Orientation is
fixed to the player's color — no flip control.

Pinned CDN ESM (no npm, browser network only):

```html
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/cm-chessboard@8.6.0/assets/chessboard.css">
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/cm-chessboard@8.6.0/assets/extensions/markers/markers.css">
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/cm-chessboard@8.6.0/assets/extensions/arrows/arrows.css">
<script type="module">
  import {Chessboard, COLOR, INPUT_EVENT_TYPE, FEN} from "https://cdn.jsdelivr.net/npm/cm-chessboard@8.6.0/src/Chessboard.js";
  import {Arrows, ARROW_TYPE} from "https://cdn.jsdelivr.net/npm/cm-chessboard@8.6.0/src/extensions/arrows/Arrows.js";
  import {Chess} from "https://cdn.jsdelivr.net/npm/chess.js@1.4.0/dist/esm/chess.js";
</script>
```

- Board: `new Chessboard(el, {position: fen, orientation: playerColor, assetsUrl: "https://cdn.jsdelivr.net/npm/cm-chessboard@8.6.0/assets/", extensions: [{class: Arrows}]})`. Pieces drag in explore mode only; main-line browsing uses `setPosition(fen, true)` animation. Display update order in `showPly`: eval bar + text first, arrows in try/catch (a failed arrow must never starve the bar).
- Arrows per shown ply: played move (secondary) + best move colored by class (Best success, Good info, Inaccuracy/Mistake warning, Blunder/MissedWin danger). `removeArrows()` before redraw.
- Eval bar: white win% height + score text, updated on every ply change AND every explore result (explore JSON carries `evalCp`/`mateIn`).
- Eval graph: hand-rolled SVG polyline, no chart lib. Per-ply white win% = mate ? 100/0 : `50+50*(2/(1+exp(-0.004*cp))-1)`. Click dot/curve -> jump. Critical plies get dots.
- Review mode: "Start review" iterates `critical[]`, sets board + arrow + coach text; Prev/Next step within criticals while in review mode (single-ply stepping outside it); Exit returns to free browse. Keyboard left/right match the buttons. Move-list clicks always exit review.
- Coach notes appear on critical positions only (Mistake/Blunder/Missed Win); other plies show move + score, no prose.
- Explore: "Try moves" toggle loads current FEN into chess.js, `enableMoveInput`; on `validateMoveInput` apply (promotion auto-queen), POST `/explore {fen, san}` (FEN from JS memory, never displayed), show returned verdict line + best arrow + updated bar. Toggle off restores main line.
- FEN rule: FENs live in `window.REVIEW` JSON and JS vars only. No input, no text node, no URL param ever shows a FEN.

## 2. review.json

```json
{
  "plies": [{"ply":1,"san":"e4","fen":"...","evalCp":25,"mateIn":0,"best":"e5","bestUci":"e7e5","cpLoss":5,"class":"Best","explain":"Best move."}],
  "whiteAcc": 82.1, "blackAcc": 74.0, "yourAcc": 82.1,
  "counts": [{"class":"Best","n":12}, ...],
  "critical": [14, 23]
}
```

Built from the transient run snapshot + replayed FENs.

## 3. Explanations (`analyze.Explain`, pure + tested)

Template sentences from class + eval delta (scores white-relative in text):

- Best (+mover mates): "Best move — mates in N." / else "Best move."
- Good: "Good — keeps the balance."
- Inaccuracy: "Inaccuracy — {best} was a touch better ({before} → {after})."
- Mistake: "Mistake — {best} kept more ({before} → {after})."
- Blunder: "Blunder — {best} {was winning|kept the edge} ({before} → {after})."
- MissedWin: mate version "Missed win — {best} mated in N." else "Missed win — {best} kept a winning edge ({before} → {after})."

## 4. Parsers (unchanged)

PGN: strip `[...]` tags anywhere on line, `{...}`, `;` comments, `$n`, move numbers (incl. attached `1.e4`), result markers (kept as result). CSV `white,black` strict rules per existing tests.

## 5. Engine (unchanged)

Raw UCI, Threads 1, fixed depth. `score cp/mate` side-to-move-relative -> converted White-relative in client.

## 6. Classification (unchanged v1)

cp-loss mover-relative: ≤10 Best, ≤30 Good, ≤70 Inaccuracy, ≤150 Mistake, else Blunder. MissedWin override (mate ≤5 dropped, or 300+ → sub-150). Accuracy `100*avg(1/(1+loss/100))`.

## 7. Build order (review rebuild)

1. `analyze`: PlyResult += Before/Fen/BestUCI + `Explain` + tests.
2. Handler `review.json` + hide FEN in explore partial.
3. `review.js` widget + game shell rewrite; drop server board table.
4. Smoke: import -> analyze -> widget (arrows, graph, review walk, explore drag).
