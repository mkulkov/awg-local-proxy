package coreclient

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Supervisor manages only its own network-free mock child. Real core activation
// must add provenance verification and Windows kill-on-close before use.
type Supervisor struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	in     io.WriteCloser
	out    *bufio.Reader
	done   chan struct{}
	seq    uint64
	closed bool
}

func StartMock(executable string, expectedSHA256 string) (*Supervisor, error) {
	if !filepath.IsAbs(executable) {
		return nil, errors.New("core path must be absolute")
	}
	b, err := os.ReadFile(executable)
	if err != nil {
		return nil, errors.New("cannot read demo executable")
	}
	h := sha256.Sum256(b)
	if expectedSHA256 == "" || hex.EncodeToString(h[:]) != expectedSHA256 {
		return nil, errors.New("demo executable hash mismatch")
	}
	cmd := exec.Command(executable, "--mock-core")
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		in.Close()
		return nil, err
	}
	// No configuration, credentials or destinations are placed in args/env/logs.
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		in.Close()
		out.Close()
		return nil, err
	}
	s := &Supervisor{cmd: cmd, in: in, out: bufio.NewReader(out), done: make(chan struct{})}
	go func() { _ = cmd.Wait(); close(s.done) }()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	hv, err := s.Hello(ctx)
	if err != nil || hv.ProtocolVersion != ProtocolVersion || hv.CoreVersion != "mock-0.1.0" {
		s.Close()
		return nil, errors.New("demo core handshake failed")
	}
	return s, nil
}

func (s *Supervisor) request(ctx context.Context, kind string, target any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("core is stopped")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.seq++
	id := strconv.FormatUint(s.seq, 10)
	type result struct {
		m   Message
		err error
	}
	ch := make(chan result, 1)
	go func() {
		if err := WriteMessage(s.in, kind, id, struct{}{}); err != nil {
			ch <- result{err: err}
			return
		}
		m, err := ReadMessage(s.out)
		ch <- result{m, err}
	}()
	select {
	case <-ctx.Done():
		s.closed = true
		s.in.Close()
		_ = s.cmd.Process.Kill()
		return ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return errors.New("demo core pipe closed")
		}
		if r.m.ID != id || r.m.Type == "error" {
			return errors.New("demo core rejected request")
		}
		want := "status"
		if kind == "hello" || kind == "shutdown" {
			want = kind
		}
		if r.m.Type != want {
			return errors.New("unexpected core reply")
		}
		if err := json.Unmarshal(r.m.Payload, target); err != nil {
			return errors.New("invalid core reply")
		}
		if st, ok := target.(*Status); ok {
			if !st.Demo {
				return errors.New("expected demo core")
			}
			return st.Validate()
		}
		return nil
	}
}

func (s *Supervisor) Hello(ctx context.Context) (Hello, error) {
	var v Hello
	err := s.request(ctx, "hello", &v)
	return v, err
}
func (s *Supervisor) Connect(ctx context.Context) (Status, error) {
	var v Status
	err := s.request(ctx, "connect", &v)
	return v, err
}
func (s *Supervisor) Disconnect(ctx context.Context) (Status, error) {
	var v Status
	err := s.request(ctx, "disconnect", &v)
	return v, err
}
func (s *Supervisor) Status(ctx context.Context) (Status, error) {
	var v Status
	err := s.request(ctx, "get_status", &v)
	return v, err
}
func (s *Supervisor) Close() error {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		_ = s.in.Close()
	}
	s.mu.Unlock()
	select {
	case <-s.done:
		return nil
	case <-time.After(2 * time.Second):
		_ = s.cmd.Process.Kill()
		<-s.done
		return errors.New("demo core required termination")
	}
}
