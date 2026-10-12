package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"chess-app/internal/analyze"
	"chess-app/web/templates"

	"github.com/gin-gonic/gin"
	"github.com/notnil/chess"
)

// StartAnalyze runs a fresh transient analysis (ADR-4). 409 when busy (ADR-5).
func (a *App) StartAnalyze(c *gin.Context) {
	id, err := gameID(c)
	if err != nil {
		c.String(http.StatusBadRequest, "bad id")
		return
	}
	g, moves, err := a.loadGame(id)
	if err != nil {
		c.String(http.StatusNotFound, "game not found")
		return
	}
	if p := a.analyzer.Progress(); p.Running {
		c.String(http.StatusConflict, "engine busy, try again when the current run finishes")
		return
	}
	rs := &runState{}
	a.setRun(id, rs)
	go func() {
		sum, aerr := a.analyzer.Analyze(context.Background(), moves, g.PlayerColor, rs.append)
		rs.mu.Lock()
		defer rs.mu.Unlock()
		rs.summary, rs.runErr, rs.finished = sum, aerr, true
	}()
	c.Redirect(http.StatusSeeOther, "/games/"+strconv.FormatUint(uint64(id), 10))
}

// AnalyzeStatus is polled by htmx (ADR-7) and returns the analyze pane partial.
func (a *App) AnalyzeStatus(c *gin.Context) {
	id, err := gameID(c)
	if err != nil {
		c.String(http.StatusBadRequest, "bad id")
		return
	}
	_, moves, err := a.loadGame(id)
	if err != nil {
		c.String(http.StatusNotFound, "game not found")
		return
	}
	rs := a.getRun(id)
	if rs == nil {
		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		_ = templates.StatusPartial(id, templates.StatusView{State: "none", Total: len(moves)}, false).Render(c.Request.Context(), c.Writer)
		return
	}
	results, summary, runErr, finished := rs.snapshot()
	st := statusOf(results, summary, runErr, finished, moves)
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = templates.StatusPartial(id, st, true).Render(c.Request.Context(), c.Writer)
}

// Explore evaluates one alternate move from a FEN (transient, never saved).
// 409 while a full analysis run occupies the engine.
func (a *App) Explore(c *gin.Context) {
	if p := a.analyzer.Progress(); p.Running {
		c.String(http.StatusConflict, "engine busy, try again when the current run finishes")
		return
	}
	fen := c.PostForm("fen")
	san := c.PostForm("san")
	opt, err := chess.FEN(fen)
	if err != nil {
		c.String(http.StatusBadRequest, "bad fen")
		return
	}
	g := chess.NewGame(opt)
	if err := g.MoveStr(san); err != nil {
		c.String(http.StatusBadRequest, "illegal move: "+san)
		return
	}
	newFEN := g.FEN()
	ev, err := a.eval.EvalFEN(c.Request.Context(), newFEN, a.depth)
	if err != nil {
		c.String(http.StatusInternalServerError, "engine failed: "+err.Error())
		return
	}
	best := analyze.UciToSAN(newFEN, ev.BestMove)
	line := "After " + san + ": " + evalText(ev.Cp, ev.MateIn) + ", best " + best
	if len(ev.PV) > 1 {
		line += " (pv: " + strings.Join(ev.PV, " ") + ")"
	}
	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(http.StatusOK, gin.H{"fen": newFEN, "line": line, "best": best, "evalCp": ev.Cp, "mateIn": ev.MateIn})
		return
	}
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = templates.ExplorePartial(newFEN, line).Render(c.Request.Context(), c.Writer)
}

// --- view helpers ---

func resultAt(results []analyze.PlyResult, ply int) (analyze.PlyResult, bool) {
	for _, r := range results {
		if r.Ply == ply {
			return r, true
		}
	}
	return analyze.PlyResult{}, false
}

var classOrder = []analyze.Class{
	analyze.Best, analyze.Good, analyze.Inaccuracy,
	analyze.Mistake, analyze.Blunder, analyze.MissedWin,
}

func statusOf(results []analyze.PlyResult, summary *analyze.Summary, runErr error, finished bool, moves []string) templates.StatusView {
	rows := moveRows(moves, results)
	switch {
	case runErr != nil:
		msg := runErr.Error()
		if errors.Is(runErr, analyze.ErrBusy) {
			msg = "engine was busy, press Analyze again"
		}
		return templates.StatusView{State: "error", Rows: rows, Total: len(moves), Done: len(results), Error: msg}
	case finished && summary != nil:
		counts := make([]templates.ClassCount, 0, len(classOrder))
		for _, cl := range classOrder {
			counts = append(counts, templates.ClassCount{Class: string(cl), N: summary.Counts[cl]})
		}
		return templates.StatusView{State: "done", Rows: rows, Total: len(moves), Done: len(results), Summary: &templates.SummaryView{
			WhiteAcc: summary.WhiteAccuracy, BlackAcc: summary.BlackAccuracy,
			YourAcc: summary.YourAccuracy, Counts: counts, Critical: summary.Critical,
		}}
	default:
		return templates.StatusView{State: "running", Rows: rows, Total: len(moves), Done: len(results)}
	}
}
