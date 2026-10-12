package chessparse

import (
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/notnil/chess"
)

// ParseError is an actionable import failure: which move/row and what value.
type ParseError struct {
	Kind   string // "pgn" or "csv"
	MoveNo int    // 1-based fullmove number (0 if not applicable, e.g. bad header)
	Ply    int    // 1-based ply (0 if not applicable)
	Col    string // "white" | "black" | "" (csv only)
	Value  string // offending SAN or cell content
	Msg    string
}

func (e *ParseError) Error() string {
	if e.MoveNo > 0 {
		return fmt.Sprintf("%s: move %d%s: %q: %s", e.Kind, e.MoveNo, colSuffix(e.Col), e.Value, e.Msg)
	}
	return fmt.Sprintf("%s: %q: %s", e.Kind, e.Value, e.Msg)
}

func colSuffix(col string) string {
	if col == "" {
		return ""
	}
	return " (" + col + ")"
}

// Result holds a successfully parsed game: ordered SAN moves + result.
type Result struct {
	Moves  []string // single ordered SAN array, White-first per ply
	Result string   // "1-0" | "0-1" | "1/2-1/2" | "*"
}

var (
	commentRe = regexp.MustCompile(`\{[^}]*\}`)
	tagRe     = regexp.MustCompile(`\[[^]]*\]`)
	nagRe     = regexp.MustCompile(`\$\d+`)
	annotRe   = regexp.MustCompile(`[!?]+$`)
	resultRe  = regexp.MustCompile(`^(1-0|0-1|1/2-1/2|\*)$`)
)

// ParsePGN parses full PGN text (headers + movetext) into ordered SAN moves.
// Comments, NAGs ($n) and annotations (!?) are stripped; legality is
// validated move by move so failures report move number + SAN.
func ParsePGN(text string) (*Result, error) {
	tokens, result, err := tokenizeMovetext(text)
	if err != nil {
		return nil, err
	}
	game := chess.NewGame()
	sans := make([]string, 0, len(tokens))
	for i, tok := range tokens {
		ply := i + 1
		moveNo := (ply + 1) / 2
		if err := game.MoveStr(tok); err != nil {
			col := "white"
			if ply%2 == 0 {
				col = "black"
			}
			return nil, &ParseError{Kind: "pgn", MoveNo: moveNo, Ply: ply, Col: col, Value: tok, Msg: "illegal move"}
		}
		sans = append(sans, tok)
	}
	return &Result{Moves: canonicalSAN(game, sans), Result: result}, nil
}

// ParseCSV parses the custom `white,black` CSV format into ordered SAN moves.
// Each row is one fullmove: `e4,e5`. A missing black cell is allowed only on
// the last non-empty row. Result is always "*" (unknown).
func ParseCSV(r io.Reader) (*Result, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	records, err := cr.ReadAll()
	if err != nil {
		return nil, &ParseError{Kind: "csv", Value: "", Msg: "unreadable csv: " + err.Error()}
	}
	if len(records) == 0 || !isHeader(records[0]) {
		got := ""
		if len(records) > 0 {
			got = strings.Join(records[0], ",")
		}
		return nil, &ParseError{Kind: "csv", Value: got, Msg: `header must be exactly "white,black"`}
	}
	rows := records[1:]
	// Drop trailing fully-empty rows (common in Excel exports).
	for len(rows) > 0 && isEmptyRow(rows[len(rows)-1]) {
		rows = rows[:len(rows)-1]
	}

	game := chess.NewGame()
	sans := []string{}
	for i, row := range rows {
		moveNo := i + 1
		white := cell(row, 0)
		black := cell(row, 1)
		if white == "" && black == "" {
			return nil, &ParseError{Kind: "csv", MoveNo: moveNo, Value: "", Msg: "empty row in the middle of the game"}
		}
		if white == "" {
			return nil, &ParseError{Kind: "csv", MoveNo: moveNo, Ply: moveNo*2 - 1, Col: "white", Value: black, Msg: "white cell is empty but black played"}
		}
		if black == "" && i != len(rows)-1 {
			return nil, &ParseError{Kind: "csv", MoveNo: moveNo, Ply: moveNo * 2, Col: "black", Value: "", Msg: "black cell is empty before the last row"}
		}
		for j, san := range []string{white, black} {
			if san == "" {
				continue
			}
			ply := (moveNo-1)*2 + j + 1
			col := "white"
			if j == 1 {
				col = "black"
			}
			if err := game.MoveStr(san); err != nil {
				return nil, &ParseError{Kind: "csv", MoveNo: moveNo, Ply: ply, Col: col, Value: san, Msg: "illegal move"}
			}
			sans = append(sans, san)
		}
	}
	return &Result{Moves: canonicalSAN(game, sans), Result: "*"}, nil
}

// tokenizeMovetext strips PGN headers, comments, semicolon comments, NAGs,
// move numbers and result markers, returning clean SAN tokens in play order
// plus the declared result marker ("1-0" | "0-1" | "1/2-1/2" | "*", default "*").
func tokenizeMovetext(text string) ([]string, string, error) {
	lines := strings.Split(text, "\n")
	var sb strings.Builder
	for _, ln := range lines {
		ln = tagRe.ReplaceAllString(ln, " ") // [White "..."] pairs, anywhere on the line
		if idx := strings.Index(ln, ";"); idx >= 0 {
			ln = ln[:idx] // semicolon comment to end of line
		}
		sb.WriteString(ln)
		sb.WriteString(" ")
	}
	body := commentRe.ReplaceAllString(sb.String(), " ")
	body = nagRe.ReplaceAllString(body, " ")
	var tokens []string
	result := "*"
	for _, field := range strings.Fields(body) {
		if resultRe.MatchString(field) {
			result = field
			continue
		}
		// Move number, possibly attached to the move ("1.", "1...", "1.e4").
		if idx := strings.LastIndex(field, "."); idx >= 0 {
			field = field[idx+1:]
			if field == "" {
				continue
			}
		}
		field = annotRe.ReplaceAllString(field, "")
		if field == "" {
			continue
		}
		tokens = append(tokens, field)
	}
	if len(tokens) == 0 {
		return nil, "*", &ParseError{Kind: "pgn", Value: "", Msg: "no moves found"}
	}
	return tokens, result, nil
}

// canonicalSAN re-encodes played moves via the pre-move positions so stored
// SAN is canonical (e.g. "0-0" -> "O-O") regardless of input spelling.
func canonicalSAN(game *chess.Game, fallback []string) []string {
	history := game.MoveHistory()
	if len(history) != len(fallback) {
		return fallback
	}
	notation := chess.AlgebraicNotation{}
	out := make([]string, len(history))
	for i, h := range history {
		out[i] = notation.Encode(h.PrePosition, h.Move)
	}
	return out
}

func isHeader(row []string) bool {
	if len(row) != 2 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(row[0]), "white") &&
		strings.EqualFold(strings.TrimSpace(row[1]), "black")
}

func isEmptyRow(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func cell(row []string, i int) string {
	if i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}
