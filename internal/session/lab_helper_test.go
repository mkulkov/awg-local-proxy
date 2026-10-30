//go:build windows

package session

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/mkulkov/awg-local-proxy/internal/coreclient"
)

// Selected only by the separate core's synthetic lab. Generated keys arrive
// through stdin, never the environment, command line or files.
func TestRealSupervisorLabHelper(t *testing.T) {
	if f := flag.Lookup("test.run"); f == nil || f.Value.String() != "^TestRealSupervisorLabHelper$" {
		t.Skip("invoked only by the separate synthetic AWG lab")
	}
	r := bufio.NewReaderSize(os.Stdin, coreclient.MaxMessageBytes+1)
	line, err := r.ReadSlice('\n')
	if err != nil || len(line) > coreclient.MaxMessageBytes {
		t.Fatal("invalid synthetic test input")
	}
	var input struct {
		CoreExecutable string `json:"core_executable"`
		CoreSHA256     string `json:"core_sha256"`
		Connect        struct {
			Config       string `json:"config"`
			SocksAddress string `json:"socks_address"`
			HTTPAddress  string `json:"http_address"`
			ProbeURL     string `json:"probe_url"`
		} `json:"connect"`
	}
	if json.Unmarshal(line, &input) != nil {
		t.Fatal("invalid synthetic test input")
	}
	clear(line)
	config := []byte(input.Connect.Config)
	input.Connect.Config = ""
	defer clear(config)
	factory := func(context.Context) (Backend, error) {
		return coreclient.StartReal(input.CoreExecutable, input.CoreSHA256)
	}
	controller := New(factory, func(string) ([]byte, error) { return append([]byte(nil), config...), nil }, nil)
	defer controller.Close()
	if _, err = controller.Start(Selection{ProfileID: "synthetic", SocksAddress: input.Connect.SocksAddress, HTTPAddress: input.Connect.HTTPAddress, ProbeURL: input.Connect.ProbeURL}); err != nil {
		t.Fatal("test session could not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	for {
		status, err := controller.Status(ctx)
		if err != nil || status.State == coreclient.Error {
			t.Fatal("test tunnel failed")
		}
		if status.State == coreclient.Connected {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("test tunnel timed out")
		case <-time.After(50 * time.Millisecond):
		}
	}
	if _, err = fmt.Fprintln(os.Stdout, `{"ready":true}`); err != nil {
		t.Fatal("cannot report test readiness")
	}
	_, _ = io.Copy(io.Discard, r)
}
