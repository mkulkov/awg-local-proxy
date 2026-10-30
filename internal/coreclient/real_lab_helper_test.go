//go:build windows

package coreclient

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
)

// This helper is explicitly selected by the separate core's synthetic lab.
// Keys arrive only through stdin and are never written to disk or arguments.
func TestRealSupervisorLabHelper(t *testing.T) {
	if f := flag.Lookup("test.run"); f == nil || f.Value.String() != "^TestRealSupervisorLabHelper$" {
		t.Skip("invoked only by the separate synthetic AWG lab")
	}
	reader := bufio.NewReaderSize(os.Stdin, MaxMessageBytes+1)
	line, err := reader.ReadSlice('\n')
	if err != nil || len(line) > MaxMessageBytes {
		t.Fatal("invalid synthetic test input")
	}
	var input struct {
		CoreExecutable string `json:"core_executable"`
		CoreSHA256     string `json:"core_sha256"`
		Connect        struct {
			Config         string `json:"config"`
			SocksAddress   string `json:"socks_address"`
			HTTPAddress    string `json:"http_address"`
			ProbeURL       string `json:"probe_url"`
			TimeoutSeconds int    `json:"timeout_seconds"`
		} `json:"connect"`
	}
	if json.Unmarshal(line, &input) != nil {
		t.Fatal("invalid synthetic test input")
	}
	clear(line)
	client, err := StartReal(input.CoreExecutable, input.CoreSHA256)
	if err != nil {
		t.Fatal("test supervisor could not start core")
	}
	defer client.Close()
	config := []byte(input.Connect.Config)
	input.Connect.Config = ""
	defer clear(config)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err := client.ValidateConfig(ctx, config); err != nil {
		t.Fatal("synthetic profile rejected")
	}
	status, err := client.ConnectProfile(ctx, ConnectRequest{Config: config, SocksAddress: input.Connect.SocksAddress, HTTPAddress: input.Connect.HTTPAddress, ProbeURL: input.Connect.ProbeURL, TimeoutSeconds: input.Connect.TimeoutSeconds})
	if err != nil || status.State != Connected {
		t.Fatal("synthetic tunnel not verified")
	}
	clear(config)
	if _, err := fmt.Fprintln(os.Stdout, `{"ready":true}`); err != nil {
		t.Fatal("cannot report test readiness")
	}
	_, _ = io.Copy(io.Discard, reader)
}
