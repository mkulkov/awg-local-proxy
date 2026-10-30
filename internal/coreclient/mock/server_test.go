package mock

import (
	"bufio"
	"bytes"
	"encoding/json"
	"testing"

	"github.com/mkulkov/awg-local-proxy/internal/coreclient"
)

func TestMockNeverClaimsVPN(t *testing.T) {
	var in, out bytes.Buffer
	for _, kind := range []string{"hello", "connect", "get_status", "disconnect", "shutdown"} {
		if err := coreclient.WriteMessage(&in, kind, kind, struct{}{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := Serve(&in, &out); err != nil {
		t.Fatal(err)
	}
	r := bufio.NewReader(&out)
	for i := 0; i < 5; i++ {
		m, err := coreclient.ReadMessage(r)
		if err != nil {
			t.Fatal(err)
		}
		if m.Type == "status" {
			var s coreclient.Status
			if err := json.Unmarshal(m.Payload, &s); err != nil {
				t.Fatal(err)
			}
			if !s.Demo || s.Validate() != nil || s.ListenerReady || s.TunnelVerified || s.State == coreclient.Connected {
				t.Fatal("mock claimed VPN")
			}
		}
	}
}

func TestMockRejectsConfigurationWithoutReflection(t *testing.T) {
	var in, out bytes.Buffer
	_ = coreclient.WriteMessage(&in, "connect", "1", map[string]string{"configuration": "SYNTHETIC_SECRET_MARKER"})
	if err := Serve(&in, &out); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out.Bytes(), []byte("SYNTHETIC_SECRET_MARKER")) {
		t.Fatal("reflected input")
	}
	m, err := coreclient.ReadMessage(bufio.NewReader(&out))
	if err != nil || m.Type != "error" {
		t.Fatal("configuration accepted")
	}
}
