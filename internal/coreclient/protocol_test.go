package coreclient

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type shortWriter struct{}

func (shortWriter) Write(b []byte) (int, error) { return len(b) - 1, nil }

func TestWriteMessageRejectsTruncatedWrite(t *testing.T) {
	if err := WriteMessage(shortWriter{}, "hello", "1", struct{}{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("truncated IPC write accepted: %v", err)
	}
}

func TestEnvelopeLimits(t *testing.T) {
	for _, s := range []string{
		`{"protocol_version":2,"type":"hello","id":"1","payload":{}}`,
		`{"protocol_version":1,"type":"hello","id":"1","payload":{},"extra":1}`,
		`{"protocol_version":1,"type":"hello","id":"","payload":{}}`,
		strings.Repeat("x", MaxMessageBytes+1),
		`{"protocol_version":1,"type":"hello","id":"1","payload":{}} {}`,
		`{"protocol_version":1,"protocol_version":1,"type":"hello","id":"1","payload":{}}`,
		`{"Protocol_version":1,"type":"hello","id":"1","payload":{}}`,
	} {
		if _, err := ReadMessage(bufio.NewReader(strings.NewReader(s + "\n"))); err == nil {
			t.Fatal("accepted invalid envelope")
		}
	}
	var b bytes.Buffer
	if err := WriteMessage(&b, "hello", "1", struct{}{}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadMessage(bufio.NewReader(&b)); err != nil {
		t.Fatal(err)
	}
}

func TestConnectedRequiresEvidence(t *testing.T) {
	now := time.Now()
	cases := []Status{{State: Connected}, {State: Connected, ListenerReady: true, TunnelVerified: true}, {State: Connected, Demo: true, ListenerReady: true, TunnelVerified: true, HandshakeAt: &now}, {State: Degraded, Demo: true, ListenerReady: true}}
	for _, s := range cases {
		if s.Validate() == nil {
			t.Fatalf("invalid state accepted: %+v", s)
		}
	}
	if err := (Status{State: Connected, ListenerReady: true, TunnelVerified: true, HandshakeAt: &now}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func FuzzReadMessage(f *testing.F) {
	f.Add(`{"protocol_version":1,"type":"hello","id":"1","payload":{}}`)
	f.Add("{invalid}")
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > MaxMessageBytes+2 {
			t.Skip()
		}
		_, _ = ReadMessage(bufio.NewReader(strings.NewReader(s + "\n")))
	})
}
