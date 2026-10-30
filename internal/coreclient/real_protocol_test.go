package coreclient

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRealConfigIsAJSONStringAndFrameIsBounded(t *testing.T) {
	config := []byte("SYNTHETIC\nquoted=\"value\"\r\ncontrol=\x01\\end")
	quoted := quoteConfig(config)
	var decoded string
	if err := json.Unmarshal(quoted, &decoded); err != nil || decoded != string(config) {
		t.Fatal("config JSON string roundtrip failed")
	}
	var b bytes.Buffer
	payload := struct {
		Config json.RawMessage `json:"config"`
	}{quoted}
	if err := writeRealMessage(&b, "validate_config", "1", payload); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b.Bytes(), []byte(`"config":"SYNTHETIC`)) {
		t.Fatal("config was not encoded as a JSON string")
	}
	b.Reset()
	large := quoteConfig(bytes.Repeat([]byte{'\n'}, MaxMessageBytes/3))
	if err := writeRealMessage(&b, "validate_config", "1", struct {
		Config json.RawMessage `json:"config"`
	}{large}); err == nil {
		t.Fatal("oversized escaped config accepted")
	}
	if b.Len() != 0 {
		t.Fatal("oversized secret frame partially written")
	}
}

func TestRealPayloadRejectsAliasesAndDuplicates(t *testing.T) {
	for _, input := range []string{`{"valid":true,"valid":true}`, `{"Valid":true}`, `{"valid":true,"secret":"SYNTHETIC"}`, `null`} {
		var result struct {
			Valid bool `json:"valid"`
		}
		if decodeRealPayload([]byte(input), &result) == nil {
			t.Fatal("ambiguous payload accepted")
		}
	}
}

func TestRealConnectRequiresLoopbackAndExplicitProbe(t *testing.T) {
	valid := ConnectRequest{Config: []byte("SYNTHETIC"), SocksAddress: "127.0.0.1:1080", ProbeURL: "https://example.invalid/check"}
	if err := valid.validate(); err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{"0.0.0.0:1080", "localhost:1080", "[::]:1080", "127.0.0.1:0", "127.0.0.1:65536"} {
		v := valid
		v.SocksAddress = address
		if v.validate() == nil {
			t.Fatal("unsafe listener accepted")
		}
	}
	for _, probe := range []string{"", "file:///tmp/test", "https://user:password@example.invalid/"} {
		v := valid
		v.ProbeURL = probe
		if v.validate() == nil {
			t.Fatal("invalid probe accepted")
		}
	}
	if strings.Contains(safeRealMessage(Connected), "SYNTHETIC") {
		t.Fatal("unsafe message")
	}
}
