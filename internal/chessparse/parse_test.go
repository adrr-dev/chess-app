package chessparse

import (
	"strings"
	"testing"
)

const samplePGN = `[White "Alice"]
[Black "Bob"]
[Result "1-0"]

1. e4 e5 2. Nf3 Nc6 3. Bb5 {old line} a6 4. Ba4 $1 Nf6 5. O-O Be7 1-0
`

func TestParsePGN(t *testing.T) {
	res, err := ParsePGN(samplePGN)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"e4", "e5", "Nf3", "Nc6", "Bb5", "a6", "Ba4", "Nf6", "O-O", "Be7"}
	if len(res.Moves) != len(want) {
		t.Fatalf("got %d moves %v, want %d", len(res.Moves), res.Moves, len(want))
	}
	for i := range want {
		if res.Moves[i] != want[i] {
			t.Fatalf("move %d: got %q, want %q (all %v)", i+1, res.Moves[i], want[i], res.Moves)
		}
	}
	if res.Result != "1-0" {
		t.Fatalf("result: got %q, want 1-0", res.Result)
	}
}

func TestParsePGNSingleLineHeaders(t *testing.T) {
	// chess.com-style export with all tags on one line.
	res, err := ParsePGN(`[White "A"] [Black "B"] [Result "1-0"] 1. e4 e5 2. Qh5 Nc6 3. Bc4 Nf6 4. Qxf7# 1-0`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"e4", "e5", "Qh5", "Nc6", "Bc4", "Nf6", "Qxf7#"}
	if strings.Join(res.Moves, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v", res.Moves)
	}
	if res.Result != "1-0" {
		t.Fatalf("result %q", res.Result)
	}
}

func TestParsePGNIllegalMove(t *testing.T) {
	_, err := ParsePGN("1. e4 e5 2. Nf3 e5")
	perr, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T (%v)", err, err)
	}
	if perr.MoveNo != 2 || perr.Col != "black" || perr.Value != "e5" {
		t.Fatalf("unexpected error detail: %+v", perr)
	}
}

func TestParsePGNEmpty(t *testing.T) {
	if _, err := ParsePGN("[White \"x\"]\n\n"); err == nil {
		t.Fatal("expected error for moveless PGN")
	}
}

func TestParseCSV(t *testing.T) {
	res, err := ParseCSV(strings.NewReader("white,black\ne4,e5\nNf3,Nc6\nBb5,a6\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"e4", "e5", "Nf3", "Nc6", "Bb5", "a6"}
	if strings.Join(res.Moves, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", res.Moves, want)
	}
}

func TestParseCSVMissingFinalBlack(t *testing.T) {
	res, err := ParseCSV(strings.NewReader("white,black\ne4,e5\nNf3,\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Join(res.Moves, ",") != "e4,e5,Nf3" {
		t.Fatalf("got %v", res.Moves)
	}
}

func TestParseCSVBadHeader(t *testing.T) {
	if _, err := ParseCSV(strings.NewReader("w,b\ne4,e5\n")); err == nil {
		t.Fatal("expected header error")
	}
}

func TestParseCSVEmptyWhite(t *testing.T) {
	_, err := ParseCSV(strings.NewReader("white,black\ne4,e5\n,Nc6\n"))
	perr, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if perr.MoveNo != 2 || perr.Col != "white" {
		t.Fatalf("unexpected error detail: %+v", perr)
	}
}

func TestParseCSVGapBlack(t *testing.T) {
	// Empty black before the last row means a skipped move.
	if _, err := ParseCSV(strings.NewReader("white,black\ne4,\nNf3,Nc6\n")); err == nil {
		t.Fatal("expected gap error")
	}
}
