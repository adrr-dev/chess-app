package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"chess-app/internal/chessparse"
	"chess-app/internal/models"
	"chess-app/web/templates"

	"github.com/gin-gonic/gin"
	"github.com/notnil/chess"
)

// Index renders the import form + history list.
func (a *App) Index(c *gin.Context) {
	var games []models.Game
	a.db.Order("created_at desc").Limit(50).Find(&games)
	rows := make([]templates.GameRow, 0, len(games))
	for _, g := range games {
		rows = append(rows, a.gameRow(&g))
	}
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = templates.IndexPage(rows).Render(c.Request.Context(), c.Writer)
}

func (a *App) gameRow(g *models.Game) templates.GameRow {
	var moves []string
	_ = json.Unmarshal([]byte(g.MovesJSON), &moves)
	return templates.GameRow{
		ID: g.ID, Source: g.Source, Result: g.Result,
		PlayerColor: g.PlayerColor, Moves: len(moves),
		Created: g.CreatedAt.Format("2006-01-02 15:04"),
	}
}

// CreateGame accepts a PGN/CSV file upload or pasted text, parses it,
// stores the moveset, and redirects to the game page.
func (a *App) CreateGame(c *gin.Context) {
	color := c.PostForm("player_color")
	if color != "white" && color != "black" {
		color = "unknown"
	}

	var res *chessparse.Result
	var source string
	if err := func() error {
		f, hdr, ferr := c.Request.FormFile("file")
		if ferr == nil {
			defer f.Close()
			data, rerr := io.ReadAll(io.LimitReader(f, 2<<20))
			if rerr != nil {
				return rerr
			}
			switch strings.ToLower(filepath.Ext(hdr.Filename)) {
			case ".csv":
				source = "csv"
				r, perr := chessparse.ParseCSV(strings.NewReader(string(data)))
				res = r
				return perr
			default:
				source = "pgn"
				r, perr := chessparse.ParsePGN(string(data))
				res = r
				return perr
			}
		}
		if t := strings.TrimSpace(c.PostForm("pgn_text")); t != "" {
			source = "pgn"
			r, perr := chessparse.ParsePGN(t)
			res = r
			return perr
		}
		if t := strings.TrimSpace(c.PostForm("csv_text")); t != "" {
			source = "csv"
			r, perr := chessparse.ParseCSV(strings.NewReader(t))
			res = r
			return perr
		}
		return errNoInput
	}(); err != nil {
		c.String(http.StatusBadRequest, "import failed: %v", messageOf(err))
		return
	}

	raw, _ := json.Marshal(res.Moves)
	g := models.Game{Source: source, PgnText: importText(c, source), MovesJSON: string(raw), PlayerColor: color, Result: res.Result}
	if err := a.db.Create(&g).Error; err != nil {
		c.String(http.StatusInternalServerError, "save failed: %v", err)
		return
	}
	c.Redirect(http.StatusSeeOther, "/games/"+strconv.FormatUint(uint64(g.ID), 10))
}

// ShowGame renders the review shell: the widget boots from embedded JSON.
func (a *App) ShowGame(c *gin.Context) {
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
	ply := len(moves)
	if q := c.Query("ply"); q != "" {
		if n, nerr := strconv.Atoi(q); nerr == nil {
			ply = n
		}
	}
	if ply < 0 {
		ply = 0
	}
	if ply > len(moves) {
		ply = len(moves)
	}
	fens, err := fensOf(moves)
	if err != nil {
		c.String(http.StatusInternalServerError, "replay failed: %v", err)
		return
	}
	view := templates.GameView{
		Game: a.gameRow(g), Ply: ply, TotalPlies: len(moves),
		Orientation: orientationOf(g.PlayerColor),
		Boot: templates.ReviewBoot{
			GameID: id, Ply: ply, Orientation: orientationOf(g.PlayerColor),
			Fens: fens, Moves: moves, Review: nil,
			ExploreURL: "/games/" + strconv.FormatUint(uint64(id), 10) + "/explore",
		},
	}
	// Attach transient analysis if a run exists.
	if rs := a.getRun(id); rs != nil {
		results, summary, runErr, finished := rs.snapshot()
		view.Status = statusOf(results, summary, runErr, finished, moves)
		if finished && runErr == nil && summary != nil {
			view.Boot.Review = buildReview(results, summary)
		}
	} else {
		view.Status = templates.StatusView{State: "none", Total: len(moves), Rows: moveRows(moves, nil)}
	}
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = templates.GamePage(view).Render(c.Request.Context(), c.Writer)
}

// fensOf replays the game and returns the FEN after each ply, index 0 = start.
func fensOf(moves []string) ([]string, error) {
	g := chess.NewGame()
	fens := make([]string, 0, len(moves)+1)
	fens = append(fens, g.FEN())
	for _, san := range moves {
		if err := g.MoveStr(san); err != nil {
			return nil, err
		}
		fens = append(fens, g.FEN())
	}
	return fens, nil
}

// DeleteGame removes a game (and drops any transient run).
func (a *App) DeleteGame(c *gin.Context) {
	id, err := gameID(c)
	if err != nil {
		c.String(http.StatusBadRequest, "bad id")
		return
	}
	a.db.Delete(&models.Game{}, id)
	a.setRun(id, nil)
	c.Redirect(http.StatusSeeOther, "/")
}

var errNoInput = errorString("no input: upload a file or paste PGN/CSV text")

type errorString string

func (e errorString) Error() string { return string(e) }

func messageOf(err error) string {
	if perr, ok := err.(*chessparse.ParseError); ok {
		return perr.Error()
	}
	return err.Error()
}

func importText(c *gin.Context, source string) string {
	if source == "csv" {
		return strings.TrimSpace(c.PostForm("csv_text"))
	}
	return strings.TrimSpace(c.PostForm("pgn_text"))
}
