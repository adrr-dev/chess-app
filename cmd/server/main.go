package main

import (
	"os"
	"strconv"

	"chess-app/internal/db"
	"chess-app/internal/engine"
	"chess-app/internal/handler"

	"github.com/gin-gonic/gin"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	dbPath := env("DB_PATH", "data/chess.db")
	port := env("PORT", "8080")
	stockfish := env("STOCKFISH_PATH", "stockfish")
	depth, err := strconv.Atoi(env("ANALYSIS_DEPTH", "18"))
	if err != nil || depth <= 0 {
		depth = 18
	}

	g, err := db.Open(dbPath)
	if err != nil {
		panic(err)
	}
	eng, err := engine.New(stockfish)
	if err != nil {
		panic(err)
	}
	defer eng.Close()

	app := handler.New(g, eng, depth)
	r := gin.Default()
	r.Static("/static", "./web/static")
	app.Register(r)
	if err := r.Run(":" + port); err != nil {
		panic(err)
	}
}
