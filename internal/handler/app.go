package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"

	"chess-app/internal/analyze"
	"chess-app/internal/engine"
	"chess-app/internal/models"
	"chess-app/web/templates"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// lockedEval serializes all engine use (analysis runs + one-shot explores)
// onto one UCI proc.
type lockedEval struct {
	mu sync.Mutex
	c  *engine.Client
}

func (l *lockedEval) EvalFEN(ctx context.Context, fen string, depth int) (*engine.Eval, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.c.EvalFEN(ctx, fen, depth)
}

// runState holds one transient analysis run (ADR-4: never persisted).
type runState struct {
	mu       sync.Mutex
	results  []analyze.PlyResult
	summary  *analyze.Summary
	runErr   error
	finished bool
}

func (r *runState) append(pr analyze.PlyResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results = append(r.results, pr)
}

func (r *runState) snapshot() (results []analyze.PlyResult, summary *analyze.Summary, runErr error, finished bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]analyze.PlyResult{}, r.results...), r.summary, r.runErr, r.finished
}

// App wires DB + engine + analyzer + routes.
type App struct {
	db       *gorm.DB
	analyzer *analyze.Service
	eval     *lockedEval
	depth    int

	runsMu sync.Mutex
	runs   map[uint]*runState
}

func New(db *gorm.DB, eng *engine.Client, depth int) *App {
	le := &lockedEval{c: eng}
	return &App{db: db, analyzer: analyze.NewService(le, depth), eval: le, depth: depth, runs: map[uint]*runState{}}
}

func (a *App) Register(r *gin.Engine) {
	r.GET("/", a.Index)
	r.GET("/health", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.POST("/games", a.CreateGame)
	r.GET("/games/:id", a.ShowGame)
	r.DELETE("/games/:id", a.DeleteGame)
	r.POST("/games/:id/delete", a.DeleteGame) // HTML form fallback
	r.POST("/games/:id/analyze", a.StartAnalyze)
	r.GET("/games/:id/analyze/status", a.AnalyzeStatus)
	r.GET("/games/:id/review.json", a.ReviewJSON)
	r.POST("/games/:id/explore", a.Explore)
}

// --- shared helpers ---

func (a *App) loadGame(id uint) (*models.Game, []string, error) {
	var g models.Game
	if err := a.db.First(&g, id).Error; err != nil {
		return nil, nil, err
	}
	var moves []string
	if err := json.Unmarshal([]byte(g.MovesJSON), &moves); err != nil {
		return nil, nil, fmt.Errorf("stored moves corrupt: %w", err)
	}
	return &g, moves, nil
}

func orientationOf(playerColor string) string {
	if playerColor == "black" {
		return "black"
	}
	return "white"
}

// evalText formats a white-relative score like chess.com: "+1.2", "-0.4", "M5", "-M3".
func evalText(cp, mate int) string {
	if mate > 0 {
		return fmt.Sprintf("M%d", mate)
	}
	if mate < 0 {
		return fmt.Sprintf("-M%d", -mate)
	}
	v := float64(cp) / 100.0
	if v >= 0 {
		return fmt.Sprintf("+%.1f", v)
	}
	return fmt.Sprintf("%.1f", v)
}

// evalPct maps a white-relative score to 0-100 bar height (100 = white best).
func evalPct(cp, mate int) float64 {
	if mate > 0 {
		return 100
	}
	if mate < 0 {
		return 0
	}
	return 50 + 50*(2/(1+math.Exp(-0.004*float64(cp)))-1)
}

func gameID(c *gin.Context) (uint, error) {
	n, err := strconv.Atoi(c.Param("id"))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("bad id")
	}
	return uint(n), nil
}

func (a *App) getRun(id uint) *runState {
	a.runsMu.Lock()
	defer a.runsMu.Unlock()
	return a.runs[id]
}

func (a *App) setRun(id uint, rs *runState) {
	a.runsMu.Lock()
	defer a.runsMu.Unlock()
	a.runs[id] = rs
}

// moveRows pairs plies into chess.com-style fullmove rows.
func moveRows(moves []string, results []analyze.PlyResult) []templates.MoveRow {
	byPly := map[int]analyze.PlyResult{}
	for _, r := range results {
		byPly[r.Ply] = r
	}
	var rows []templates.MoveRow
	for i := 0; i < len(moves); i += 2 {
		row := templates.MoveRow{N: i/2 + 1}
		row.White = templates.PlyCell{Ply: i + 1, San: moves[i]}
		if r, ok := byPly[i+1]; ok {
			row.White.Eval, row.White.Class = evalText(r.EvalCp, r.MateIn), string(r.Class)
		}
		if i+1 < len(moves) {
			row.Black = &templates.PlyCell{Ply: i + 2, San: moves[i+1]}
			if r, ok := byPly[i+2]; ok {
				row.Black.Eval, row.Black.Class = evalText(r.EvalCp, r.MateIn), string(r.Class)
			}
		}
		rows = append(rows, row)
	}
	return rows
}
