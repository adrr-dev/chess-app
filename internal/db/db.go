package db

import (
	"os"
	"path/filepath"

	"chess-app/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Open opens SQLite (creating parent dirs) and AutoMigrates Game.
// Postgres swap later = change Dialector here (ADR-2).
func Open(path string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	g, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := g.AutoMigrate(&models.Game{}); err != nil {
		return nil, err
	}
	return g, nil
}
