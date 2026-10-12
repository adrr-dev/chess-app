package templates

// GameRow is one history entry.
type GameRow struct {
	ID          uint
	Source      string
	Result      string
	PlayerColor string
	Moves       int
	Created     string
}

// PlyCell is one analyzed (or bare) half-move in the move list.
type PlyCell struct {
	Ply   int
	San   string
	Eval  string // "" when not analyzed
	Class string // "" when not analyzed
}

// MoveRow pairs white/black plies into one fullmove row.
type MoveRow struct {
	N     int
	White PlyCell
	Black *PlyCell // nil on trailing white move
}

// ClassCount is one summary bucket, display-ordered.
type ClassCount struct {
	Class string
	N     int
}

// SummaryView is the overall analysis header.
type SummaryView struct {
	WhiteAcc float64
	BlackAcc float64
	YourAcc  float64
	Counts   []ClassCount
	Critical []int
}

// StatusView is the analyze pane state.
type StatusView struct {
	State   string // "none" | "running" | "done" | "error"
	Done    int
	Total   int
	Rows    []MoveRow
	Summary *SummaryView
	Error   string
}

// ReviewBoot is the widget's initial state, embedded via templ.JSONScript.
// FENs travel here only — never as visible text (ADR-9).
type ReviewBoot struct {
	GameID      uint     `json:"gameID"`
	Ply         int      `json:"ply"`
	Orientation string   `json:"orientation"`
	Fens        []string `json:"fens"`
	Moves       []string `json:"moves"`
	Review      any      `json:"review"`
	ExploreURL  string   `json:"exploreURL"`
}

// GameView is the full game page model.
type GameView struct {
	Game        GameRow
	Ply         int
	TotalPlies  int
	Orientation string
	Status      StatusView
	Boot        ReviewBoot
}
