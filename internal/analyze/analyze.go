// Package analyze classifies moves from engine evals and runs full-game
// analysis as a single-flight service (ADR-4 transient, ADR-5 single-flight).
package analyze

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"chess-app/internal/engine"

	"github.com/notnil/chess"
)

// Class is a chess.com-style move verdict.
type Class string

const (
	Best       Class = "Best"
	Good       Class = "Good"
	Inaccuracy Class = "Inaccuracy"
	Mistake    Class = "Mistake"
	Blunder    Class = "Blunder"
	MissedWin  Class = "Missed Win"
)

// Thresholds on centipawn loss from the mover's perspective (DESIGN.md §4).
const (
	threshBest       = 10
	threshGood       = 30
	threshInaccuracy = 70
	threshMistake    = 150
	winCp            = 300
	winDropCp        = 150
	shortMate        = 5
)

// PlyResult is the transient per-move verdict (never persisted).
type PlyResult struct {
	Ply        int    // 1-based
	San        string // played move (canonical SAN)
	EvalCp     int    // white-relative cp after the move (0 when decisive mate)
	MateIn     int    // white-relative mate after the move (0 = none)
	Best       string // best move, SAN (falls back to UCI)
	BestUCI    string // best move, UCI (for board arrows)
	BeforeCp   int    // white-relative cp before the move (== best-line eval)
	BeforeMate int    // white-relative mate before the move (0 = none)
	Fen        string // position after the move
	CpLoss     int    // mover-relative, clamped >= 0
	Class      Class
}

// Summary aggregates one analysis run.
type Summary struct {
	Plies         []PlyResult
	Counts        map[Class]int
	WhiteAccuracy float64
	BlackAccuracy float64
	YourAccuracy  float64 // filtered by playerColor
	Critical      []int   // plies classed Mistake or worse
}

// Evaluator is satisfied by *engine.Client; fakes allowed in tests.
type Evaluator interface {
	EvalFEN(ctx context.Context, fen string, depth int) (*engine.Eval, error)
}

// moverView converts a white-relative eval to the mover's perspective:
// mate>0 means the mover mates in mate, mate<0 the mover is mated in -mate.
func moverView(ev engine.Eval, whiteToMove bool) (mate int, cp int) {
	mate, cp = ev.MateIn, ev.Cp
	if !whiteToMove {
		mate, cp = -mate, -cp
	}
	return mate, cp
}

// ClassifyPlayed verdicts the played move. best/played are white-relative
// engine evals of the pre- and post-move positions.
func ClassifyPlayed(best, played engine.Eval, whiteToMove bool) Class {
	bM, bC := moverView(best, whiteToMove)
	pM, pC := moverView(played, whiteToMove)

	if pM > 0 {
		return Best // played move delivers mate
	}
	if pM < 0 {
		return Blunder // played move walks into mate
	}
	if bM > 0 {
		// Best line mates, played line doesn't.
		if bM <= shortMate {
			return MissedWin
		}
		if pC >= winDropCp {
			return Inaccuracy // stayed comfortably winning
		}
		return Blunder
	}
	loss := bC - pC
	if loss < 0 {
		loss = 0 // played better than best at this depth; engine noise
	}
	if bC >= winCp && pC < winDropCp {
		return MissedWin
	}
	switch {
	case loss <= threshBest:
		return Best
	case loss <= threshGood:
		return Good
	case loss <= threshInaccuracy:
		return Inaccuracy
	case loss <= threshMistake:
		return Mistake
	default:
		return Blunder
	}
}

// CpLossOf returns the mover-relative centipawn loss used for accuracy math.
// Mate transitions map to fixed penalties (kept/found = 0).
func CpLossOf(best, played engine.Eval, whiteToMove bool) int {
	bM, bC := moverView(best, whiteToMove)
	pM, pC := moverView(played, whiteToMove)
	switch {
	case pM > 0:
		return 0
	case pM < 0:
		return 1000
	case bM > 0:
		if bM <= shortMate || pC < winDropCp {
			return winCp
		}
		return threshInaccuracy
	default:
		if loss := bC - pC; loss > 0 {
			return loss
		}
		return 0
	}
}

// Accuracy is the chess.com-style 0-100 score over per-ply losses.
func Accuracy(losses []int) float64 {
	if len(losses) == 0 {
		return 0
	}
	sum := 0.0
	for _, l := range losses {
		sum += 1.0 / (1.0 + float64(l)/100.0)
	}
	return 100.0 * sum / float64(len(losses))
}

// scoreText formats a white-relative score like the UI: "+1.2", "-0.4", "M5".
func scoreText(cp, mate int) string {
	if mate > 0 {
		return fmt.Sprintf("M%d", mate)
	}
	if mate < 0 {
		return fmt.Sprintf("-M%d", -mate)
	}
	if v := float64(cp) / 100.0; v >= 0 {
		return fmt.Sprintf("+%.1f", v)
	} else {
		return fmt.Sprintf("%.1f", v)
	}
}

// mateForMover reports whether the white-relative mate score means the mover
// delivers mate, and in how many.
func mateForMover(mate int, whiteToMove bool) (dist int, ok bool) {
	if whiteToMove && mate > 0 {
		return mate, true
	}
	if !whiteToMove && mate < 0 {
		return -mate, true
	}
	return 0, false
}

// Explain renders a one-line coach verdict for a ply (ADR-11). Deterministic
// template text from class + eval delta; no LLM.
func Explain(p PlyResult, whiteToMove bool) string {
	before := scoreText(p.BeforeCp, p.BeforeMate)
	after := scoreText(p.EvalCp, p.MateIn)
	if dist, ok := mateForMover(p.MateIn, whiteToMove); ok && (p.Class == Best || p.Class == Good) {
		return fmt.Sprintf("%s — mates in %d.", p.Class, dist)
	}
	switch p.Class {
	case Best:
		return "Best move."
	case Good:
		return "Good — keeps the balance."
	case Inaccuracy:
		return fmt.Sprintf("Inaccuracy — %s was a touch better (%s → %s).", p.Best, before, after)
	case Mistake:
		return fmt.Sprintf("Mistake — %s kept more (%s → %s).", p.Best, before, after)
	case Blunder:
		if p.BeforeCp >= winCp && whiteToMove || p.BeforeCp <= -winCp && !whiteToMove {
			return fmt.Sprintf("Blunder — threw away a winning position (%s → %s). %s was best.", before, after, p.Best)
		}
		return fmt.Sprintf("Blunder — %s kept the edge (%s → %s).", p.Best, before, after)
	case MissedWin:
		if dist, ok := mateForMover(p.BeforeMate, whiteToMove); ok {
			return fmt.Sprintf("Missed win — %s mated in %d.", p.Best, dist)
		}
		return fmt.Sprintf("Missed win — %s kept a winning edge (%s → %s).", p.Best, before, after)
	default:
		return string(p.Class) + "."
	}
}

// ErrBusy is returned when an analysis run is already in flight.
var ErrBusy = errors.New("analyze: another analysis is already running")

// Progress is the poll state for the status endpoint (ADR-7).
type Progress struct {
	Running bool
	Done    int
	Total   int
}

// Service runs full-game analysis, one run at a time.
type Service struct {
	eng   Evaluator
	depth int

	mu      sync.Mutex
	running bool
	done    int
	total   int
}

func NewService(eng Evaluator, depth int) *Service {
	return &Service{eng: eng, depth: depth}
}

func (s *Service) Progress() Progress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Progress{Running: s.running, Done: s.done, Total: s.total}
}

// Analyze replays moves, evals every ply from scratch (ADR-4) and streams
// verdicts via onPly (nil allowed). Second concurrent call gets ErrBusy.
// The mutex guards state only (the run itself doesn't hold it, so Progress
// stays readable); single-flight is enforced via the running flag.
func (s *Service) Analyze(ctx context.Context, moves []string, playerColor string, onPly func(PlyResult)) (*Summary, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil, ErrBusy
	}
	s.running = true
	s.done = 0
	s.total = len(moves)
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()
	bump := func() {
		s.mu.Lock()
		s.done++
		s.mu.Unlock()
	}

	preFENs, postFENs, sans, decisive, err := replay(moves)
	if err != nil {
		return nil, err
	}

	sum := &Summary{Counts: map[Class]int{}, Critical: []int{}}
	var whiteLosses, blackLosses []int
	for i := range moves {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ply := i + 1
		whiteToMove := ply%2 == 1

		best, err := s.eng.EvalFEN(ctx, preFENs[i], s.depth)
		if err != nil {
			return nil, fmt.Errorf("ply %d best eval: %w", ply, err)
		}
		played, err := s.eng.EvalFEN(ctx, postFENs[i], s.depth)
		if err != nil {
			return nil, fmt.Errorf("ply %d played eval: %w", ply, err)
		}
		if decisive[i] {
			// Mover delivered mate; engines report this as mate 0, which
			// reads as "no mate" — override with a mate-for-mover score.
			played = &engine.Eval{BestMove: played.BestMove, PV: played.PV, Depth: played.Depth}
			if whiteToMove {
				played.MateIn = 1
			} else {
				played.MateIn = -1
			}
		}

		loss := CpLossOf(*best, *played, whiteToMove)
		class := ClassifyPlayed(*best, *played, whiteToMove)
		if whiteToMove {
			whiteLosses = append(whiteLosses, loss)
		} else {
			blackLosses = append(blackLosses, loss)
		}
		pr := PlyResult{
			Ply: ply, San: sans[i],
			EvalCp: played.Cp, MateIn: played.MateIn,
			Best: UciToSAN(preFENs[i], best.BestMove), BestUCI: best.BestMove,
			BeforeCp: best.Cp, BeforeMate: best.MateIn,
			Fen:    postFENs[i],
			CpLoss: loss, Class: class,
		}
		sum.Plies = append(sum.Plies, pr)
		sum.Counts[class]++
		if class == Mistake || class == Blunder || class == MissedWin {
			sum.Critical = append(sum.Critical, ply)
		}
		if onPly != nil {
			onPly(pr)
		}
		bump()
	}

	sum.WhiteAccuracy = Accuracy(whiteLosses)
	sum.BlackAccuracy = Accuracy(blackLosses)
	switch playerColor {
	case "white":
		sum.YourAccuracy = sum.WhiteAccuracy
	case "black":
		sum.YourAccuracy = sum.BlackAccuracy
	default:
		sum.YourAccuracy = (sum.WhiteAccuracy + sum.BlackAccuracy) / 2
	}
	return sum, nil
}

// replay validates moves in order and returns pre/post FENs, canonical SAN,
// and whether each move decisively ended the game (mover delivered mate).
// The last matters because engines report post-mate positions as mate 0,
// which is indistinguishable from "no mate" on the wire.
func replay(moves []string) (pre, post, sans []string, decisive []bool, err error) {
	g := chess.NewGame()
	notation := chess.AlgebraicNotation{}
	for i, san := range moves {
		pre = append(pre, g.FEN())
		if err := g.MoveStr(san); err != nil {
			ply := i + 1
			return nil, nil, nil, nil, &replayError{ply: ply, san: san, err: err}
		}
		// Exact pre-position pair from history (no index guessing).
		h := g.MoveHistory()[len(g.MoveHistory())-1]
		sans = append(sans, notation.Encode(h.PrePosition, h.Move))
		post = append(post, g.FEN())
		decisive = append(decisive, g.Outcome() == chess.WhiteWon || g.Outcome() == chess.BlackWon)
	}
	return pre, post, sans, decisive, nil
}

type replayError struct {
	ply int
	san string
	err error
}

func (e *replayError) Error() string {
	return fmt.Sprintf("analyze: illegal move at ply %d %q: %v", e.ply, e.san, e.err)
}

// UciToSAN maps an engine UCI move to SAN in the given position.
// Falls back to the raw UCI string when no legal move matches.
func UciToSAN(fen, uci string) string {
	if uci == "" {
		return ""
	}
	opt, err := chess.FEN(fen)
	if err != nil {
		return uci
	}
	g := chess.NewGame(opt)
	notation := chess.AlgebraicNotation{}
	for _, m := range g.ValidMoves() {
		if m.String() == uci {
			return notation.Encode(g.Position(), m)
		}
	}
	return uci
}
