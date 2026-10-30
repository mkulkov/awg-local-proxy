package profiles

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

func syntheticVPN(t testing.TB, native string) []byte {
	t.Helper()
	client, err := json.Marshal(map[string]any{"config": native, "client_priv_key": "SYNTHETIC_CLIENT_METADATA"})
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := json.Marshal(map[string]any{
		"format_version": 1, "description": "SYNTHETIC_DESCRIPTION",
		"userName": "SYNTHETIC_SSH_USER", "password": "SYNTHETIC_SSH_PASSWORD", "port": 22,
		"containers": []any{map[string]any{"container": "amnezia-awg", "awg": map[string]any{
			"last_config": string(client), "server_priv_key": "SYNTHETIC_SERVER_KEY",
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return envelope
}

func syntheticQCompress(t testing.TB, input []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(input)))
	b.Write(header[:])
	w := zlib.NewWriter(&b)
	if _, err := w.Write(input); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestVPNKnownTransportsExtractOnlyClient(t *testing.T) {
	envelope := syntheticVPN(t, "# SYNTHETIC_COMMENT_SECRET\n"+syntheticConfig())
	compressed := syntheticQCompress(t, envelope)
	for name, input := range map[string][]byte{
		"JSON":             envelope,
		"qCompress":        compressed,
		"base64 JSON":      []byte(base64.RawURLEncoding.EncodeToString(envelope)),
		"base64 qCompress": []byte(base64.RawURLEncoding.EncodeToString(compressed)),
		"URI":              []byte("vpn://" + base64.RawURLEncoding.EncodeToString(compressed)),
		"padded URI":       []byte("vpn://" + base64.URLEncoding.EncodeToString(compressed)),
		"file newline":     []byte("\xef\xbb\xbf vpn://" + base64.RawURLEncoding.EncodeToString(compressed) + "\r\n"),
	} {
		t.Run(name, func(t *testing.T) {
			result, err := ImportClientConfig(input)
			if err != nil {
				t.Fatal(err)
			}
			defer clear(result.Config)
			if string(result.Config) != syntheticConfig() || result.Source != ImportSourceAmneziaVPN || !result.DiscardedMetadata {
				t.Fatal("only native client configuration and metadata warning must be returned")
			}
			if bytes.Contains(result.Config, []byte("SYNTHETIC_")) {
				t.Fatal("discarded secret metadata escaped into result")
			}
		})
	}
	result, err := ImportClientConfig([]byte(syntheticConfig()))
	if err != nil || result.Source != ImportSourceConf || result.DiscardedMetadata {
		t.Fatal("native import compatibility changed")
	}
	clear(result.Config)
}

func TestVPNContainerIdentityAndLegacyVersion(t *testing.T) {
	envelope := string(syntheticVPN(t, syntheticConfig()))
	for _, input := range []string{
		strings.Replace(envelope, `"format_version":1,`, "", 1),
		strings.Replace(envelope, "amnezia-awg", "amnezia-awg2", 1),
		strings.Replace(envelope, `"containers":[`, `"containers":[{"container":"amnezia-openvpn","openvpn":{}},`, 1),
	} {
		result, err := ImportClientConfig([]byte(input))
		if err != nil {
			t.Fatal("known unambiguous client export rejected")
		}
		clear(result.Config)
	}
}

func TestVPNRejectsAmbiguityAndUnsupportedStructures(t *testing.T) {
	envelope := string(syntheticVPN(t, syntheticConfig()))
	var root map[string]any
	if err := json.Unmarshal([]byte(envelope), &root); err != nil {
		t.Fatal(err)
	}
	containers := root["containers"].([]any)
	root["containers"] = append(containers, containers[0])
	duplicate, _ := json.Marshal(root)
	for _, input := range [][]byte{
		duplicate,
		[]byte(`{"containers":[],"password":"SYNTHETIC_SECRET"}`),
		[]byte(`{"containers":[{"container":"amnezia-awg","awg":{}}]}`),
		[]byte(`{"config":"SYNTHETIC_SECRET"}`),
		[]byte(`{"api_key":"SYNTHETIC_SECRET","auth_data":{}}`),
		[]byte(strings.Replace(envelope, `"format_version":1`, `"format_version":2`, 1)),
		[]byte(strings.Replace(envelope, `"format_version":1`, `"format_version":"1"`, 1)),
		[]byte(strings.Replace(envelope, "amnezia-awg", "unknown-awg", 1)),
		[]byte(strings.Replace(envelope, `"containers":`, `"Containers":[],"containers":`, 1)),
		[]byte(envelope + envelope),
		syntheticVPN(t, strings.Replace(syntheticConfig(), "[Peer]", "SSHPassword = SYNTHETIC_SECRET\n[Peer]", 1)),
		syntheticVPN(t, syntheticConfig()+"[Peer]\n"),
		syntheticVPN(t, "#"+strings.Repeat("x", MaxImportBytes)+"\n"+syntheticConfig()),
	} {
		result, err := ImportClientConfig(input)
		if err == nil || len(result.Config) != 0 {
			t.Fatal("ambiguous, administrative or unsupported configuration accepted")
		}
		if strings.Contains(err.Error(), "SYNTHETIC_") {
			t.Fatal("secret reflected in error")
		}
	}
}

func TestVPNRejectsDuplicateNestedJSON(t *testing.T) {
	envelope := string(syntheticVPN(t, syntheticConfig()))
	for _, input := range []string{
		strings.Replace(envelope, `"awg":{`, `"awg":{"last_config":"{}",`, 1),
		strings.Replace(envelope, `\"config\":`, `\"Config\":\"ambiguous\",\"config\":`, 1),
		strings.Replace(envelope, `"password":`, `"password":"first","password":`, 1),
	} {
		if _, err := ImportClientConfig([]byte(input)); err == nil {
			t.Fatal("duplicate JSON key accepted")
		}
	}
}

func TestVPNCompressionAndInputBounds(t *testing.T) {
	valid := syntheticQCompress(t, syntheticVPN(t, syntheticConfig()))
	wrongSize := bytes.Clone(valid)
	binary.BigEndian.PutUint32(wrongSize[:4], binary.BigEndian.Uint32(wrongSize[:4])+1)
	tooLarge := bytes.Clone(valid)
	binary.BigEndian.PutUint32(tooLarge[:4], MaxVPNDecodedBytes+1)
	corrupt := bytes.Clone(valid)
	corrupt[len(corrupt)-1] ^= 1
	for _, input := range [][]byte{
		nil, []byte("vpn://"), []byte("vpn://A"), []byte("vpn://AA==?secret"),
		[]byte("vpn://AA\nAA"), []byte("vpn://AA+_"), []byte("vpn://AB"),
		[]byte(strings.Repeat("x", MaxVPNInputBytes+1)),
		valid[:len(valid)-1], append(bytes.Clone(valid), 0), append(bytes.Clone(valid), valid[4:]...),
		wrongSize, tooLarge, corrupt, {0, 0, 0, 0, 0x78, 0x9c},
		syntheticQCompress(t, bytes.Repeat([]byte("x"), MaxVPNDecodedBytes+1)),
		[]byte("{\"metadata\":" + strings.Repeat("[", 18) + "0" + strings.Repeat("]", 18) + "}"),
		[]byte("{\"bad\":\"\xff\"}"),
	} {
		if result, err := ImportClientConfig(input); err == nil || len(result.Config) != 0 {
			t.Fatal("corrupt or unbounded envelope accepted")
		}
	}
}

func TestVPNJSONParserResourceLimits(t *testing.T) {
	for _, input := range []string{
		`{"metadata":[` + strings.Repeat("0,", 16384) + `0]}`,
		`{"metadata":` + strings.Repeat("[", 18) + "0" + strings.Repeat("]", 18) + `}`,
		`{"` + strings.Repeat("k", 129) + `":0}`,
	} {
		if _, err := parseVPNObject([]byte(input)); err == nil {
			t.Fatal("JSON resource budget bypassed")
		}
	}
}

func FuzzImportClientConfig(f *testing.F) {
	f.Add([]byte(syntheticConfig()))
	envelope := syntheticVPN(f, syntheticConfig())
	f.Add(envelope)
	f.Add([]byte("vpn://" + base64.RawURLEncoding.EncodeToString(syntheticQCompress(f, envelope))))
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > MaxVPNInputBytes+1 {
			t.Skip()
		}
		result, err := ImportClientConfig(input)
		defer clear(result.Config)
		if err == nil {
			if len(result.Config) > MaxImportBytes || ValidateImport(result.Config) != nil {
				t.Fatal("successful envelope import bypassed client validation")
			}
		} else if len(result.Config) != 0 {
			t.Fatal("failed import returned secret bytes")
		}
	})
}
