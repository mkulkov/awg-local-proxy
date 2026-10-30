package profiles

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"
)

func syntheticConfig() string {
	k := base64.StdEncoding.EncodeToString(make([]byte, 32))
	return "[Interface]\nPrivateKey = " + k + "\nAddress = 10.0.0.2/32\nDNS = 10.0.0.1\n[Peer]\nPublicKey = " + k + "\nEndpoint = 192.0.2.1:51820\nAllowedIPs = 0.0.0.0/0\n"
}
func TestImportRejectsUnsafeOrAmbiguousConfig(t *testing.T) {
	for _, b := range []string{
		syntheticConfig() + "[Peer]\n",
		strings.Replace(syntheticConfig(), "[Peer]", "PostUp = run.exe\n[Peer]", 1),
		strings.Replace(syntheticConfig(), "[Peer]", "UnknownSecret = test\n[Peer]", 1),
		strings.Replace(syntheticConfig(), "[Peer]", "PrivateKey = duplicate\n[Peer]", 1),
		strings.Replace(syntheticConfig(), "[Peer]", "Jc = 4 # SYNTHETIC_ADMIN_SECRET\n[Peer]", 1),
		strings.Replace(syntheticConfig(), "[Peer]", "Jc = SSH_PASSWORD\n[Peer]", 1),
		strings.Replace(syntheticConfig(), "[Peer]", "S1 = SSH_PASSWORD\n[Peer]", 1),
		strings.Replace(syntheticConfig(), "[Peer]", "H1 = SSH_PASSWORD\n[Peer]", 1),
		strings.Replace(syntheticConfig(), "[Peer]", "I1 = SSH_PASSWORD\n[Peer]", 1),
		strings.Repeat("x", maxSecretBytes+1),
		`{"ssh_password":"synthetic"}`,
	} {
		if ValidateImport([]byte(b)) == nil {
			t.Fatal("unsafe config accepted")
		}
	}
	if err := ValidateImport([]byte(syntheticConfig())); err != nil {
		t.Fatal(err)
	}
}

func TestImportEndpointAddressesAndHostnames(t *testing.T) {
	for _, endpoint := range []string{
		"192.0.2.1:51820", "[2001:db8::1]:51820", "vpn.example:51820",
		"VPN-1.example.invalid:1", "localhost:65535", "xn--e1afmkfd.example:51820",
		strings.Repeat("a", 63) + ".example:51820",
		strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 61) + ":51820",
	} {
		input := []byte(strings.Replace(syntheticConfig(), "192.0.2.1:51820", endpoint, 1))
		stored, err := ClientConfigForStorage(input)
		if err != nil {
			t.Fatalf("supported endpoint rejected: %v", err)
		}
		if !strings.Contains(string(stored), "Endpoint = "+endpoint+"\n") {
			t.Fatal("endpoint was changed during import")
		}
	}
}

func TestImportRejectsMalformedEndpointsWithoutEchoingThem(t *testing.T) {
	for _, endpoint := range []string{
		":51820", "vpn.example:", "vpn.example:0", "vpn.example:65536", "vpn.example:+80",
		"vpn.example:-1", "vpn.example:80/path", "vpn.example:000080", "2001:db8::1:51820",
		"vpn..example:51820", ".vpn.example:51820", "vpn.example.:51820",
		"-vpn.example:51820", "vpn-.example:51820", "vpn_1.example:51820",
		"vpn.example :51820", "vpn.exa\tmple:51820", "vpn.example:\t51820",
		"https://vpn.example:51820", "user@vpn.example:51820", "vpn/example:51820",
		"пример.example:51820", "vpn.例子:51820", "[vpn.example]:51820",
		strings.Repeat("a", 64) + ".example:51820",
		strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 62) + ":51820",
	} {
		input := []byte(strings.Replace(syntheticConfig(), "192.0.2.1:51820", endpoint, 1))
		if err := ValidateImport(input); err == nil {
			t.Errorf("unsupported endpoint accepted: %q", endpoint)
		} else if strings.Contains(err.Error(), endpoint) {
			t.Fatal("endpoint reflected in import error")
		}
	}
}

func TestImportHostnameDoesNotResolve(t *testing.T) {
	previous := net.DefaultResolver
	var calls atomic.Int32
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(context.Context, string, string) (net.Conn, error) {
		calls.Add(1)
		return nil, errors.New("network forbidden by import test")
	}}
	t.Cleanup(func() { net.DefaultResolver = previous })
	input := []byte(strings.Replace(syntheticConfig(), "192.0.2.1:51820", "not-resolved.example.invalid:51820", 1))
	if err := ValidateImport(input); err != nil {
		t.Fatal("offline hostname validation failed")
	}
	if calls.Load() != 0 {
		t.Fatal("import attempted DNS resolution")
	}
}

func TestImportEndpointPreservesWindowsINIWhitespaceCompatibility(t *testing.T) {
	for _, endpoint := range []string{"192.0.2.1:51820", "[2001:db8::1]:51820", "vpn.example.invalid:51820"} {
		input := strings.Replace(syntheticConfig(), "192.0.2.1:51820", "\t "+endpoint+" \t ", 1)
		input = strings.ReplaceAll(input, "\n", "\r\n")
		stored, err := ClientConfigForStorage([]byte(input))
		if err != nil || !strings.Contains(string(stored), endpoint) {
			t.Fatal("Windows line endings or outer INI whitespace prevented endpoint import")
		}
	}
}

func TestStoredImportOmitsComments(t *testing.T) {
	b, err := ClientConfigForStorage([]byte("# SYNTHETIC_ADMIN_COMMENT\n" + syntheticConfig()))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "SYNTHETIC_ADMIN_COMMENT") {
		t.Fatal("comment retained")
	}
}

func TestImportObfuscationGrammarAndBudget(t *testing.T) {
	withField := func(field, value string) []byte {
		return []byte(strings.Replace(syntheticConfig(), "[Peer]", field+" = "+value+"\n[Peer]", 1))
	}
	for _, value := range []string{"<b 0x1234><t><r 8><rc 12><rd 3><d><ds><dz 2>", "<r 0>", "<r 2016>", "<t> <b abCD>"} {
		input := withField("I1", value)
		stored, err := ClientConfigForStorage(input)
		if err != nil || !strings.Contains(string(stored), "I1 = "+value) {
			t.Fatalf("supported I grammar not preserved: %v", err)
		}
	}
	for _, value := range []string{"SSH_PASSWORD", "<r -1>", "<r 4097>", "<r 2016><t>", "<r 2017>", "<dz 0>", "<dz 9>", "<r 8 SSH_PASSWORD>", "<d SSH_PASSWORD>", "<t> SSH_PASSWORD", "SSH_PASSWORD <t>", "<b 0xabc>", "<b ZZ>", "<unknown 4>", strings.Repeat("<t>", 33)} {
		if ValidateImport(withField("I1", value)) == nil {
			t.Fatal("unsafe I grammar accepted")
		}
	}
	for _, value := range []string{"1-5", "4294967295", "0-4294967294"} {
		if err := ValidateImport(withField("H1", value)); err != nil {
			t.Fatalf("supported H range rejected: %v", err)
		}
	}
	for _, value := range []string{"5-1", "0-4294967295", "1-2-3", "-1", "1-SSH_PASSWORD", "1 - 5"} {
		if ValidateImport(withField("H1", value)) == nil {
			t.Fatal("unsafe H range accepted")
		}
	}
}

func TestImportRejectsPinnedWindowsBufferOverflow(t *testing.T) {
	withFields := func(fields string) []byte {
		return []byte(strings.Replace(syntheticConfig(), "[Peer]", fields+"\n[Peer]", 1))
	}
	for _, fields := range []string{"MTU = 1985", "MTU = 1984\nS4 = 1", "S4 = 565", "S1 = 1869", "S2 = 1925", "S3 = 1953", "Jc = 129\nJmin = 1\nJmax = 2", "Jc = 1", "Jc = 1\nJmin = 2\nJmax = 2", "Jmin = 3\nJmax = 2", "Jmax = 2017"} {
		if ValidateImport(withFields(fields)) == nil {
			t.Fatal("unsafe Windows packet budget accepted")
		}
	}
	for _, fields := range []string{"MTU = 1984\nS4 = 0", "S4 = 564", "S1 = 1868\nS2 = 1924\nS3 = 1952", "Jc = 128\nJmin = 1\nJmax = 2016"} {
		if err := ValidateImport(withFields(fields)); err != nil {
			t.Fatalf("safe boundary rejected: %v", err)
		}
	}
}

func TestImportAWG31FieldsAndRanges(t *testing.T) {
	withFields := func(fields string) []byte {
		return []byte(strings.Replace(syntheticConfig(), "[Peer]", fields+"\n[Peer]", 1))
	}
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	header := "HeaderProtectionKey = " + key + "\nS1 = 12\nS2 = 16\nS3 = 20\nS4 = 24"
	for _, fields := range []string{
		header,
		header + "\nContentPaddingAddition = 540",
		header + "\nContentPaddingAddition = 8-16\nRekeyAfterTime = 120-121\nRekeyTimeout = 5-6\nRejectAfterTime = 180-181\nKeepaliveTimeout = 10-11\nMaxHandshakeAttempts = 4-5\nRandomTrailers = true\nDisableCookies = true",
		"ContentPaddingAddition = 0-564\nRekeyAfterTime = 0\nRekeyTimeout = 1-4294967295\nRandomTrailers = 1\nDisableCookies = FALSE",
		"RejectAfterTime = 1-3074457345\nMaxHandshakeAttempts = 1-4294967293",
	} {
		stored, err := ClientConfigForStorage(withFields(fields))
		if err != nil || !strings.Contains(string(stored), fields) {
			t.Fatal("supported AWG3.1 parameters changed or rejected")
		}
		clear(stored)
	}
	for _, fields := range []string{
		"HeaderProtectionKey = " + key,
		strings.Replace(header, "S3 = 20", "S3 = 11", 1),
		strings.Replace(header, key, base64.StdEncoding.EncodeToString(make([]byte, 32)), 1),
		strings.Replace(header, key, "SYNTHETIC_SECRET", 1),
		header + "\nContentPaddingAddition = 541",
		"ContentPaddingAddition = 565", "ContentPaddingAddition = 2017", "ContentPaddingAddition = 2-1",
		"ContentPaddingAddition = 0-4294967295", "ContentPaddingAddition = 1 - 2",
		"RekeyAfterTime = 0-1", "RekeyTimeout = -1", "RejectAfterTime = 2-1",
		"KeepaliveTimeout = 4294967296", "MaxHandshakeAttempts = 0-4294967295",
		"RejectAfterTime = 3074457346", "MaxHandshakeAttempts = 4294967294",
		"RandomTrailers = SYNTHETIC_SECRET", "DisableCookies = yes", "header_protection_key = " + key,
	} {
		if ValidateImport(withFields(fields)) == nil {
			t.Fatal("unsafe or unsupported AWG3.1 parameter accepted")
		}
	}
	for _, value := range []string{"0", "0-0", "20-21", "1-4294967295", "4294967295"} {
		if err := ValidateImport([]byte(syntheticConfig() + "PersistentKeepalive = " + value + "\n")); err != nil {
			t.Fatal("supported keepalive range rejected")
		}
	}
	for _, value := range []string{"0-1", "2-1", "4294967296", "0-4294967295", "SYNTHETIC_SECRET"} {
		if ValidateImport([]byte(syntheticConfig()+"PersistentKeepalive = "+value+"\n")) == nil {
			t.Fatal("unsafe keepalive range accepted")
		}
	}
}

func TestImportSizeLeavesRoomForEscapedIPC(t *testing.T) {
	input := []byte("#" + strings.Repeat("x", MaxImportBytes) + "\n" + syntheticConfig())
	if ValidateImport(input) == nil {
		t.Fatal("import exceeds safe IPC budget")
	}
	// Existing encrypted storage retains its previous size boundary.
	s, err := Open(t.TempDir(), fixtureProtector{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateEncrypted("Legacy-sized synthetic blob", []byte(strings.Repeat("x", MaxImportBytes+1))); err != nil {
		t.Fatal("legacy storage size compatibility changed")
	}
}
func FuzzValidateImport(f *testing.F) {
	f.Add(syntheticConfig())
	f.Add("vpn://bad")
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > maxSecretBytes+1 {
			t.Skip()
		}
		_ = ValidateImport([]byte(s))
	})
}
