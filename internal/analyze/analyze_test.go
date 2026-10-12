package analyze

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"chess-app/internal/engine"
)

func ev(cp, mate int) engine.Eval {
	return engine.Eval{Cp: cp, MateIn: mate}
}

func TestClassifyTable(t *testing.T) {
	cases := []struct {
		name        string
		best, play  engine.Eval
		whiteToMove bool
		want        Class
	}{
		{"equal", ev(30, 0), ev(30, 0), true, Best},
		{"loss10", ev(30, 0), ev(20, 0), true, Best},
		{"loss20", ev(30, 0), ev(10, 0), true, Good},
		{"loss50", ev(60, 0), ev(10, 0), true, Inaccuracy},
		{"loss100", ev(100, 0), ev(0, 0), true, Mistake},
		{"loss200", ev(100, 0), ev(-100, 0), true, Blunder},
		{"playedBetter", ev(30, 0), ev(80, 0), true, Best},
		{"missedWinCp", ev(400, 0), ev(100, 0), true, MissedWin},
		{"keptWinning", ev(400, 0), ev(380, 0), true, Good},
		{"mateFound", ev(200, 0), ev(0, 1), true, Best},
		{"mateDroppedShort", ev(0, 3), ev(50, 0), true, MissedWin},
		{"walkedIntoMate", ev(50, 0), ev(0, -2), true, Blunder},
		// Black to move: white-relative -100 vs -250 means black improved.
		{"blackImproved", ev(-100, 0), ev(-250, 0), false, Best},
		// Black to move: -100 to +100 means black dropped 200cp.
		{"blackBlunder", ev(-100, 0), ev(100, 0), false, Blunder},
		// Black mates in 2, plays it.
		{"blackMates", ev(0, 0), ev(0, -2), false, Best},
	}
	for _, tc := range cases {
		if got := ClassifyPlayed(tc.best, tc.play, tc.whiteToMove); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestAccuracy(t *testing.T) {
	if got := Accuracy(nil); got != 0 {
		t.Fatalf("empty: got %v", got)
	}
	if got := Accuracy([]int{0, 0, 0}); got != 100 {
		t.Fatalf("perfect: got %v", got)
	}
	// One blunder-sized loss drags it down but stays sane.
	got := Accuracy([]int{0, 200})
	if got <= 0 || got >= 100 {
		t.Fatalf("mixed out of range: %v", got)
	}
}

func TestExplain(t *testing.T) {
	mk := func(class Class, san, best string, beforeCp, beforeMate, cp, mate int) PlyResult {
		return PlyResult{San: san, Best: best, Class: class,
			BeforeCp: beforeCp, BeforeMate: beforeMate, EvalCp: cp, MateIn: mate}
	}
	cases := []struct {
		name        string
		p           PlyResult
		whiteToMove bool
		wantSub     []string
	}{
		{"best", mk(Best, "e4", "e4", 20, 0, 25, 0), true, []string{"Best move."}},
		{"bestMate", mk(Best, "Qxf7#", "Qxf7#", 300, 0, 0, 1), true, []string{"mates in 1"}},
		{"good", mk(Good, "e5", "e5", 20, 0, 15, 0), false, []string{"Good"}},
		{"inaccuracy", mk(Inaccuracy, "a3", "Nf3", 30, 0, -20, 0), true, []string{"Inaccuracy", "Nf3"}},
		{"mistake", mk(Mistake, "h3", "Nf3", 40, 0, -80, 0), true, []string{"Mistake", "Nf3", "+0.4", "-0.8"}},
		{"blunder", mk(Blunder, "f3", "Nf3", 60, 0, -200, 0), true, []string{"Blunder", "Nf3"}},
		{"blunderWin", mk(Blunder, "f3", "Qh5", 500, 0, -100, 0), true, []string{"threw away a winning"}},
		{"missedMate", mk(MissedWin, "Kg1", "Qxf7#", 0, 1, 60, 0), true, []string{"Missed win", "Qxf7#", "mated in 1"}},
		{"missedCp", mk(MissedWin, "a4", "d4", 400, 0, 100, 0), true, []string{"Missed win", "d4"}},
		{"blackBlunder", mk(Blunder, "f6", "Nf6", -60, 0, 200, 0), false, []string{"Blunder", "Nf6"}},
	}
	for _, tc := range cases {
		got := Explain(tc.p, tc.whiteToMove)
		for _, sub := range tc.wantSub {
			if !strings.Contains(got, sub) {
				t.Errorf("%s: %q missing %q", tc.name, got, sub)
			}
		}
	}
}

func TestUciToSAN(t *testing.T) {
	start := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	if got := UciToSAN(start, "e2e4"); got != "e4" {
		t.Fatalf("got %q", got)
	}
	afterE4 := "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1"
	if got := UciToSAN(afterE4, "g8f6"); got != "Nf6" {
		t.Fatalf("got %q", got)
	}
	if got := UciToSAN(start, "z9z9"); got != "z9z9" {
		t.Fatalf("fallback: got %q", got)
	}
	if got := UciToSAN(start, ""); got != "" {
		t.Fatalf("empty: got %q", got)
	}
}

// fakeEng returns a fixed eval; best UCI move configurable per test.
type fakeEng struct {
	cp   int
	best string
}

func (f fakeEng) EvalFEN(_ context.Context, _ string, _ int) (*engine.Eval, error) {
	return &engine.Eval{Cp: f.cp, BestMove: f.best, Depth: 8}, nil
}

func TestServiceRun(t *testing.T) {
	s := NewService(fakeEng{cp: 30, best: "e2e4"}, 8)
	var streamed int
	sum, err := s.Analyze(context.Background(), []string{"e4", "e5", "Nf3"}, "white", func(PlyResult) { streamed++ })
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if len(sum.Plies) != 3 || streamed != 3 {
		t.Fatalf("plies=%d streamed=%d", len(sum.Plies), streamed)
	}
	total := 0
	for _, n := range sum.Counts {
		total += n
	}
	if total != 3 {
		t.Fatalf("counts sum %d: %v", total, sum.Counts)
	}
	if sum.YourAccuracy != sum.WhiteAccuracy {
		t.Fatalf("your accuracy %v != white %v", sum.YourAccuracy, sum.WhiteAccuracy)
	}
	if p := s.Progress(); p.Running || p.Total != 3 || p.Done != 3 {
		t.Fatalf("progress after run: %+v", p)
	}
}

type blockEng struct{ release chan struct{} }

func (b blockEng) EvalFEN(ctx context.Context, _ string, _ int) (*engine.Eval, error) {
	select {
	case <-b.release:
		return &engine.Eval{Cp: 10, BestMove: "e2e4", Depth: 1}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestServiceBusy(t *testing.T) {
	eng := blockEng{release: make(chan struct{})}
	s := NewService(eng, 1)
	done := make(chan error, 1)
	go func() {
		_, err := s.Analyze(context.Background(), []string{"e4"}, "white", nil)
		done <- err
	}()
	time.Sleep(100 * time.Millisecond) // let the first run take the lock
	if _, err := s.Analyze(context.Background(), []string{"e4"}, "white", nil); !errors.Is(err, ErrBusy) {
		t.Fatalf("expected ErrBusy, got %v", err)
	}
	if p := s.Progress(); !p.Running || p.Total != 1 {
		t.Fatalf("progress during run: %+v", p)
	}
	close(eng.release) // let the first run finish; no leak
	<-done
}

func TestServiceDeliveredMate(t *testing.T) {
	// Scholar's mate: the mating move must be Best even though engines
	// report the post-mate position as mate 0 (fakes return plain cp).
	s := NewService(fakeEng{cp: 30, best: "e2e4"}, 8)
	moves := []string{"e4", "e5", "Qh5", "Nc6", "Bc4", "Nf6", "Qxf7#"}
	sum, err := s.Analyze(context.Background(), moves, "white", nil)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	last := sum.Plies[len(sum.Plies)-1]
	if last.Class != Best {
		t.Fatalf("mating move: got %q, want Best", last.Class)
	}
	if last.MateIn <= 0 {
		t.Fatalf("mating move mate score: %+v", last)
	}
}

func TestServiceIllegalMove(t *testing.T) {
	s := NewService(fakeEng{}, 1)
	if _, err := s.Analyze(context.Background(), []string{"e4", "e4"}, "white", nil); err == nil {
		t.Fatal("expected replay error")
	}
}
