package handler

import (
	"net/http"

	"chess-app/internal/analyze"
	"chess-app/web/templates"

	"github.com/gin-gonic/gin"
)

// plyJSON is one reviewed ply for the widget (DESIGN.md §2).
type plyJSON struct {
	Ply     int    `json:"ply"`
	San     string `json:"san"`
	Fen     string `json:"fen"`
	EvalCp  int    `json:"evalCp"`
	MateIn  int    `json:"mateIn"`
	Best    string `json:"best"`
	BestUCI string `json:"bestUci"`
	CpLoss  int    `json:"cpLoss"`
	Class   string `json:"class"`
	Explain string `json:"explain"`
}

// reviewPayload is the full GET review.json body.
type reviewPayload struct {
	Plies    []plyJSON              `json:"plies"`
	WhiteAcc float64                `json:"whiteAcc"`
	BlackAcc float64                `json:"blackAcc"`
	YourAcc  float64                `json:"yourAcc"`
	Counts   []templates.ClassCount `json:"counts"`
	Critical []int                  `json:"critical"`
}

func buildReview(results []analyze.PlyResult, summary *analyze.Summary) reviewPayload {
	plies := make([]plyJSON, 0, len(results))
	for _, r := range results {
		plies = append(plies, plyJSON{
			Ply: r.Ply, San: r.San, Fen: r.Fen,
			EvalCp: r.EvalCp, MateIn: r.MateIn,
			Best: r.Best, BestUCI: r.BestUCI,
			CpLoss: r.CpLoss, Class: string(r.Class),
			Explain: analyze.Explain(r, r.Ply%2 == 1),
		})
	}
	counts := make([]templates.ClassCount, 0, len(classOrder))
	for _, cl := range classOrder {
		counts = append(counts, templates.ClassCount{Class: string(cl), N: summary.Counts[cl]})
	}
	return reviewPayload{
		Plies:    plies,
		WhiteAcc: summary.WhiteAccuracy, BlackAcc: summary.BlackAccuracy,
		YourAcc: summary.YourAccuracy, Counts: counts, Critical: summary.Critical,
	}
}

// ReviewJSON serves the transient review for the board widget.
// 404 when never analyzed, 409 while a run is in flight, 500 on run error.
func (a *App) ReviewJSON(c *gin.Context) {
	id, err := gameID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad id"})
		return
	}
	_, _, lerr := a.loadGame(id)
	if lerr != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "game not found"})
		return
	}
	rs := a.getRun(id)
	if rs == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not analyzed yet"})
		return
	}
	results, summary, runErr, finished := rs.snapshot()
	if runErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": runErr.Error()})
		return
	}
	if !finished || summary == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "analysis running"})
		return
	}
	c.JSON(http.StatusOK, buildReview(results, summary))
}
