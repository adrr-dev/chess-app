// Package engine drives a Stockfish binary via raw UCI over a subprocess
// (ADR-3). One Client owns one proc; calls are sequential, not goroutine-safe.
package engine

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Eval is one engine answer, scores normalized to White-relative.
type Eval struct {
	BestMove string   // UCI, e.g. "e7e5" ("" if none)
	PV       []string // principal variation, UCI moves
	Cp       int      // white-relative centipawns (valid when MateIn == 0)
	MateIn   int      // >0 White mates, <0 Black mates, 0 = no mate
	Depth    int      // depth of the info line used
}

// Client owns a Stockfish subprocess speaking UCI. A single background
// goroutine owns stdout; all methods consume lines from c.lines.
type Client struct {
	cmd   *exec.Cmd
	in    *bufio.Writer
	lines chan string
}

// New starts `path` ("" defaults to "stockfish"), handshakes
// (uci/isready), pins Threads 1. Close with Close.
func New(path string) (*Client, error) {
	if path == "" {
		path = "stockfish"
	}
	cmd := exec.Command(path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("uci stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("uci stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w (is stockfish installed?)", path, err)
	}
	c := &Client{cmd: cmd, in: bufio.NewWriter(stdin), lines: make(chan string, 64)}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	go func() {
		for sc.Scan() {
			c.lines <- sc.Text()
		}
		close(c.lines)
	}()
	if err := c.expect("uci", "uciok", 10*time.Second); err != nil {
		cmd.Process.Kill()
		return nil, err
	}
	if err := c.send("setoption name Threads value 1"); err != nil {
		cmd.Process.Kill()
		return nil, err
	}
	if err := c.expect("isready", "readyok", 10*time.Second); err != nil {
		cmd.Process.Kill()
		return nil, err
	}
	return c, nil
}

// NewGame tells the engine a new (unrelated) analysis run starts.
func (c *Client) NewGame() error {
	if err := c.send("ucinewgame"); err != nil {
		return err
	}
	return c.expect("isready", "readyok", 10*time.Second)
}

// EvalFEN evaluates fen at fixed depth. Mate scores and cp are returned
// White-relative (UCI reports side-to-move-relative; converted here).
// Cancelling ctx sends `stop`, drains the reply, and returns ctx.Err().
func (c *Client) EvalFEN(ctx context.Context, fen string, depth int) (*Eval, error) {
	whiteToMove, err := whiteToMove(fen)
	if err != nil {
		return nil, err
	}
	if err := c.send("position fen " + fen); err != nil {
		return nil, err
	}
	if err := c.send(fmt.Sprintf("go depth %d", depth)); err != nil {
		return nil, err
	}

	ev := &Eval{}
	stopped := false
	for {
		select {
		case <-ctx.Done():
			if !stopped {
				stopped = true
				c.send("stop") // best-effort; reply still drained below
			}
		case line, ok := <-c.lines:
			if !ok {
				return nil, fmt.Errorf("uci: engine closed output")
			}
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			switch fields[0] {
			case "info":
				if info, ok := parseInfo(fields, depth); ok {
					*ev = *info
				}
			case "bestmove":
				if len(fields) > 1 && fields[1] != "(none)" {
					ev.BestMove = fields[1]
				}
				ev.toWhiteRelative(whiteToMove)
				if stopped {
					return nil, ctx.Err()
				}
				return ev, nil
			}
		}
	}
}

// Close quits the engine and waits for the proc.
func (c *Client) Close() error {
	c.send("quit")
	return c.cmd.Wait()
}

func (c *Client) send(s string) error {
	if _, err := c.in.WriteString(s + "\n"); err != nil {
		return fmt.Errorf("uci write %q: %w", s, err)
	}
	return c.in.Flush()
}

// expect sends cmd and waits for a line starting with want.
func (c *Client) expect(cmd, want string, timeout time.Duration) error {
	if err := c.send(cmd); err != nil {
		return err
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case line, ok := <-c.lines:
			if !ok {
				return fmt.Errorf("uci %q: output closed", cmd)
			}
			if line == want || strings.HasPrefix(line, want+" ") {
				return nil
			}
			// Ignore option/id lines, keep waiting.
		case <-timer.C:
			return fmt.Errorf("uci %q: timed out waiting for %q", cmd, want)
		}
	}
}

// parseInfo extracts score+pv from an info line at the requested depth.
// ok=false when the line isn't usable (wrong depth, multipv>1, no score).
func parseInfo(fields []string, depth int) (*Eval, bool) {
	ev := &Eval{}
	gotDepth, gotScore := false, false
	for i := 0; i < len(fields); i++ {
		switch fields[i] {
		case "depth":
			if i+1 < len(fields) {
				if d, err := strconv.Atoi(fields[i+1]); err == nil && d == depth {
					gotDepth = true
					ev.Depth = d
				}
			}
		case "multipv":
			if i+1 < len(fields) && fields[i+1] != "1" {
				return nil, false
			}
		case "score":
			if i+2 >= len(fields) {
				return nil, false
			}
			v, err := strconv.Atoi(fields[i+2])
			if err != nil {
				return nil, false
			}
			if fields[i+1] == "mate" {
				ev.MateIn = v // side-to-move-relative; converted later
			} else if fields[i+1] == "cp" {
				ev.Cp = v // side-to-move-relative; converted later
			} else {
				return nil, false
			}
			gotScore = true
		case "pv":
			if i+1 < len(fields) {
				ev.PV = append([]string{}, fields[i+1:]...)
			}
		}
	}
	if !gotDepth || !gotScore {
		return nil, false
	}
	return ev, true
}

// toWhiteRelative converts side-to-move-relative scores to White-relative.
func (e *Eval) toWhiteRelative(whiteToMove bool) {
	if whiteToMove {
		return
	}
	e.Cp = -e.Cp
	e.MateIn = -e.MateIn
}

func whiteToMove(fen string) (bool, error) {
	parts := strings.Split(fen, " ")
	if len(parts) < 2 {
		return false, fmt.Errorf("bad fen %q", fen)
	}
	switch parts[1] {
	case "w":
		return true, nil
	case "b":
		return false, nil
	}
	return false, fmt.Errorf("bad fen side %q", fen)
}
