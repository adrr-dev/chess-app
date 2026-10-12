import { Chessboard, COLOR, INPUT_EVENT_TYPE } from "https://cdn.jsdelivr.net/npm/cm-chessboard@8.6.0/src/Chessboard.js";
import { Arrows, ARROW_TYPE } from "https://cdn.jsdelivr.net/npm/cm-chessboard@8.6.0/src/extensions/arrows/Arrows.js";
import { Chess } from "https://cdn.jsdelivr.net/npm/chess.js@1.4.0/dist/esm/chess.js";

const R = JSON.parse(document.getElementById("review-data").textContent);
const ASSETS = "https://cdn.jsdelivr.net/npm/cm-chessboard@8.6.0/assets/";

const board = new Chessboard(document.getElementById("board"), {
  position: R.fens[R.ply],
  orientation: R.orientation === "black" ? COLOR.black : COLOR.white,
  assetsUrl: ASSETS,
  extensions: [{ class: Arrows }],
});

const chess = new Chess(); // scratch instance for SAN<->squares lookups
let ply = R.ply;
let reviewMode = false;
let reviewIdx = 0;
let exploring = false;
let exploreFen = null;

const $ = (id) => document.getElementById(id);
const byPly = {};
if (R.review) for (const p of R.review.plies) byPly[p.ply] = p;

function winPct(cp, mate) {
  if (mate > 0) return 100;
  if (mate < 0) return 0;
  return 50 + 50 * (2 / (1 + Math.exp(-0.004 * cp)) - 1);
}

function classArrow(cls) {
  switch (cls) {
    case "Best": return ARROW_TYPE.success;
    case "Good": return ARROW_TYPE.info;
    case "Inaccuracy":
    case "Mistake": return ARROW_TYPE.warning;
    default: return ARROW_TYPE.danger; // Blunder, Missed Win
  }
}

// Squares for a SAN move played from fen (null when not found).
function moveSquares(fen, san) {
  try {
    chess.load(fen);
  } catch { return null; }
  const m = chess.moves({ verbose: true }).find((x) => x.san === san);
  return m ? { from: m.from, to: m.to } : null;
}

function setEval(cp, mate) {
  const pct = winPct(cp, mate);
  $("evalfill").style.height = pct.toFixed(1) + "%";
  $("evaltext").textContent = scoreText(cp, mate);
}

function scoreText(cp, mate) {
  if (mate > 0) return "M" + mate;
  if (mate < 0) return "-M" + -mate;
  const v = cp / 100;
  return (v >= 0 ? "+" : "") + v.toFixed(1);
}

// Notes appear on critical positions only; the rest stay clean.
const CRITICAL = new Set(["Mistake", "Blunder", "Missed Win"]);

function hlMoveRow() {
  document.querySelectorAll("#movetable a.mv").forEach((a) => {
    a.classList.toggle("text-amber-300", Number(a.dataset.ply) === ply);
  });
}

function showPly(n) {
  ply = Math.max(0, Math.min(R.fens.length - 1, n));
  $("plynum").textContent = ply;
  board.disableMoveInput();
  board.setPosition(R.fens[ply], true);
  const pr = byPly[ply];
  // Bar + text first: nothing downstream may starve them.
  if (pr) {
    setEval(pr.evalCp, pr.mateIn);
    $("coach").textContent = CRITICAL.has(pr.class)
      ? pr.explain
      : R.moves[ply - 1] + " (" + scoreText(pr.evalCp, pr.mateIn) + ")";
  } else {
    setEval(20, 0);
    if (!exploring) $("coach").textContent = ply === 0 ? "Start position." : R.moves[ply - 1];
  }
  try {
    board.removeArrows();
    if (ply > 0) {
      const sq = moveSquares(R.fens[ply - 1], R.moves[ply - 1]);
      if (sq) board.addArrow(ARROW_TYPE.secondary, sq.from, sq.to);
    }
    if (pr && pr.bestUci && pr.bestUci.length >= 4) {
      board.addArrow(classArrow(pr.class), pr.bestUci.slice(0, 2), pr.bestUci.slice(2, 4));
    }
  } catch (e) { console.warn("arrows failed:", e); }
  hlMoveRow();
  drawGraphCursor();
}

// ---- eval graph (hand SVG, no chart lib) ----
function buildGraph() {
  const el = $("graph");
  if (!R.review) { el.innerHTML = ""; return; }
  const W = 300, H = 56, n = R.review.plies.length;
  let pts = R.review.plies.map((p, i) => {
    const x = (i / Math.max(1, n - 1)) * W;
    const y = H - (winPct(p.evalCp, p.mateIn) / 100) * H;
    return x.toFixed(1) + "," + y.toFixed(1);
  }).join(" ");
  const crit = new Set(R.review.critical);
  let dots = R.review.plies.filter((p) => crit.has(p.ply)).map((p) => {
    const x = ((p.ply - 1) / Math.max(1, n - 1)) * W;
    const y = H - (winPct(p.evalCp, p.mateIn) / 100) * H;
    return `<circle cx="${x.toFixed(1)}" cy="${y.toFixed(1)}" r="3" fill="#f59e0b" data-ply="${p.ply}" style="cursor:pointer"/>`;
  }).join("");
  el.innerHTML = `<svg id="graphsvg" viewBox="0 0 ${W} ${H}" class="w-full bg-stone-800 rounded" style="height:${H}px">` +
    `<line x1="0" y1="${H / 2}" x2="${W}" y2="${H / 2}" stroke="#57534e" stroke-width="1"/>` +
    `<polyline points="${pts}" fill="none" stroke="#e7e5e4" stroke-width="1.5"/>${dots}` +
    `<line id="graphcursor" y1="0" y2="${H}" stroke="#f59e0b" stroke-width="1"/></svg>`;
  el.querySelectorAll("circle").forEach((c) => {
    c.addEventListener("click", () => { exitReview(); showPly(Number(c.dataset.ply)); });
  });
  el.querySelector("#graphsvg").addEventListener("click", (e) => {
    if (e.target.tagName === "circle") return;
    const rect = e.currentTarget.getBoundingClientRect();
    const frac = (e.clientX - rect.left) / rect.width;
    exitReview();
    showPly(1 + Math.round(frac * (n - 1)));
  });
}

function drawGraphCursor() {
  const cur = document.getElementById("graphcursor");
  if (!cur || !R.review || R.review.plies.length < 2) return;
  const n = R.review.plies.length;
  cur.setAttribute("x1", ((ply - 1) / (n - 1)) * 300);
  cur.setAttribute("x2", ((ply - 1) / (n - 1)) * 300);
}

// ---- review mode (walk critical moments) ----
function enterReview() {
  const crit = (R.review && R.review.critical) || [];
  if (!R.review || crit.length === 0) {
    $("coach").textContent = R.review ? "No critical moments — clean game." : "Analyze first to review.";
    return;
  }
  reviewMode = true;
  reviewIdx = 0;
  $("btn-review").textContent = "Exit review";
  showPly(crit[0], true);
}

function exitReview() {
  reviewMode = false;
  $("btn-review").textContent = "Start review";
}

function stepReview(d) {
  if (!reviewMode || !R.review) return;
  const crit = R.review.critical || [];
  if (crit.length === 0) return;
  reviewIdx = (reviewIdx + d + crit.length) % crit.length;
  showPly(crit[reviewIdx], true);
}

// ---- explore (drag alternate moves) ----
function setExploring(on) {
  exploring = on;
  $("btn-explore").textContent = "Try moves: " + (on ? "on" : "off");
  if (on) {
    exitReview();
    exploreFen = R.fens[ply];
    try { chess.load(exploreFen); } catch { setExploring(false); return; }
    board.setPosition(exploreFen, false);
    board.enableMoveInput(inputHandler);
    $("coach").textContent = "Drag a piece to try another move.";
  } else {
    board.disableMoveInput();
    showPly(ply);
  }
}

function inputHandler(event) {
  if (event.type === INPUT_EVENT_TYPE.moveInputStarted) {
    try {
      chess.load(exploreFen);
      return chess.moves({ square: event.squareFrom, verbose: true }).length > 0;
    } catch { return false; }
  }
  if (event.type === INPUT_EVENT_TYPE.validateMoveInput) {
    let move = null;
    try {
      chess.load(exploreFen);
      try {
        move = chess.move({ from: event.squareFrom, to: event.squareTo });
      } catch {
        move = chess.move({ from: event.squareFrom, to: event.squareTo, promotion: "q" });
      }
    } catch { return false; }
    if (!move) return false;
    const san = move.san;
    const preFen = exploreFen;
    fetch(R.exploreURL, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded", "Accept": "application/json" },
      body: new URLSearchParams({ fen: preFen, san }),
    }).then((r) => r.json()).then((data) => {
      exploreFen = data.fen; // kept in JS memory, never displayed
      board.setPosition(data.fen, true);
      setEval(data.evalCp ?? 20, data.mateIn ?? 0);
      try {
        board.removeArrows();
        board.addArrow(ARROW_TYPE.secondary, move.from, move.to);
        chess.load(data.fen);
        const bm = chess.moves({ verbose: true }).find((x) => x.san === data.best);
        if (bm) board.addArrow(ARROW_TYPE.info, bm.from, bm.to);
      } catch (e) { console.warn("explore arrows failed:", e); }
      $("explore-result").textContent = data.line;
      $("coach").textContent = data.line;
    }).catch(() => { $("coach").textContent = "Engine busy or failed — try again."; });
    return true;
  }
  return true;
}

// ---- wiring ----
$("btn-prev").addEventListener("click", () => { reviewMode ? stepReview(-1) : showPly(ply - 1); });
$("btn-next").addEventListener("click", () => { reviewMode ? stepReview(1) : showPly(ply + 1); });
$("btn-review").addEventListener("click", () => { reviewMode ? exitReview() : enterReview(); });
$("btn-explore").addEventListener("click", () => setExploring(!exploring));
document.addEventListener("keydown", (e) => {
  if (e.target.tagName === "INPUT" || e.target.tagName === "TEXTAREA") return;
  if (e.key === "ArrowLeft") { reviewMode ? stepReview(-1) : (exitReview(), showPly(ply - 1)); }
  if (e.key === "ArrowRight") { reviewMode ? stepReview(1) : (exitReview(), showPly(ply + 1)); }
});
document.querySelectorAll("#movetable a.mv").forEach((a) => {
  a.addEventListener("click", (e) => { e.preventDefault(); exitReview(); showPly(Number(a.dataset.ply)); });
});

buildGraph();
showPly(ply);
