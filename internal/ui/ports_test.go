package ui

import "testing"

func TestLocalAddressOnlyAcceptsSafeLoopbackPorts(t *testing.T) {
	for _, tt := range []struct {
		in, want string
		valid    bool
	}{
		{"1", "127.0.0.1:1", true}, {"1080", "127.0.0.1:1080", true},
		{"65535", "127.0.0.1:65535", true}, {"00080", "127.0.0.1:80", true},
		{"", "", false}, {"0", "", false}, {"65536", "", false},
		{"-1", "", false}, {"+1", "", false}, {" 80", "", false},
		{"1.2.3.4:80", "", false}, {"１", "", false},
	} {
		got, err := localAddress(tt.in)
		if (err == nil) != tt.valid || got != tt.want {
			t.Errorf("localAddress(%q) = %q, %v", tt.in, got, err)
		}
	}
}

func TestLocalAddressesRespectsOptionalHTTPAndRejectsConflicts(t *testing.T) {
	socks, http, err := localAddresses("1080", "not-a-port", false)
	if err != nil || socks != "127.0.0.1:1080" || http != "" {
		t.Fatalf("disabled HTTP: %q %q %v", socks, http, err)
	}
	socks, http, err = localAddresses("1080", "8080", true)
	if err != nil || socks != "127.0.0.1:1080" || http != "127.0.0.1:8080" {
		t.Fatalf("both proxies: %q %q %v", socks, http, err)
	}
	for _, pair := range [][2]string{{"00080", "80"}, {"0", "8080"}, {"1080", "65536"}} {
		if _, _, err := localAddresses(pair[0], pair[1], true); err == nil {
			t.Errorf("accepted invalid pair %q", pair)
		}
	}
}
