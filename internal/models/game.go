package models

import "gorm.io/gorm"

// Game is the only persisted table in MVP (history = imports).
// Analysis results are transient, never stored (ADR-4).
type Game struct {
	gorm.Model
	Source      string // "pgn" | "csv"
	PgnText     string `gorm:"type:text"`
	MovesJSON   string `gorm:"type:text"` // ordered SAN array as JSON
	PlayerColor string `gorm:"index"`     // "white" | "black" | "unknown"
	Result      string
}
