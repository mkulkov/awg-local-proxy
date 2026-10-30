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
