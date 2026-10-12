package engine

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func testClient(t *testing.T) *Client {
	t.Helper()
	path := os.Getenv("STOCKFISH_PATH")
	if path == "" {
		path = "stockfish"
	}
	c, err := New(path)
	if err != nil {
		t.Skipf("stockfish not available (%v)", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestEvalStartpos(t *testing.T) {
	c := testClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ev, err := c.EvalFEN(ctx, "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", 8)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if len(ev.BestMove) < 4 {
		t.Fatalf("no bestmove: %+v", ev)
	}
	if ev.MateIn != 0 {
		t.Fatalf("startpos should have no mate: %+v", ev)
	}
	if len(ev.PV) == 0 {
		t.Fatalf("no pv: %+v", ev)
	}
	t.Logf("startpos depth 8: best=%s cp=%d pv=%s", ev.BestMove, ev.Cp, strings.Join(ev.PV, " "))
}

func TestEvalMateInOne(t *testing.T) {
	c := testClient(t)
	// Scholar's mate setup, White to play Qxf7#.
	fen := "r1bqkb1r/pppp1ppp/2n2n2/4p2Q/2B1P3/8/PPPP1PPP/RNB1K1NR w KQkq - 4 4"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ev, err := c.EvalFEN(ctx, fen, 8)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if ev.MateIn <= 0 {
		t.Fatalf("expected white mate score, got %+v", ev)
	}
	if ev.BestMove != "h5f7" {
		t.Fatalf("expected h5f7, got %+v", ev)
	}
}

func TestEvalBlackToMoveSign(t *testing.T) {
	c := testClient(t)
	// Black is missing her queen (d8 empty); white-relative cp must be
	// clearly positive even though it is Black to move
	// (UCI reports side-to-move-relative).
	fen := "rnb1kbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR b KQkq - 0 1"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ev, err := c.EvalFEN(ctx, fen, 6)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if ev.MateIn != 0 {
		t.Fatalf("unexpected mate: %+v", ev)
	}
	if ev.Cp < 300 {
		t.Fatalf("expected clearly positive white-relative cp, got %+v", ev)
	}
}

func TestEvalBadFEN(t *testing.T) {
	c := testClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := c.EvalFEN(ctx, "not-a-fen", 4); err == nil {
		t.Fatal("expected error for bad fen")
	}
}
