package profiles

import (
	"encoding/base64"
	"strings"
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
		strings.Replace(syntheticConfig(), "192.0.2.1:51820", "example.com:51820", 1),
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
