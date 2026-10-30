package profiles

import (
	"encoding/base64"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Leave room for worst-case JSON escaping inside the 64 KiB IPC envelope.
const MaxImportBytes = 8 * 1024

// ValidateImport is a conservative original client-only storage filter. It does
// not assert cryptographic validity or engine compatibility and never executes a
// hook. Import itself never resolves endpoints or starts a core connection.
func ValidateImport(input []byte) error {
	bad := func() error {
		return errors.New("конфигурация не входит в поддерживаемый безопасный поднабор .conf; файл не сохранён")
	}
	if len(input) == 0 || len(input) > MaxImportBytes || !utf8.Valid(input) {
		return bad()
	}
	text := strings.TrimPrefix(string(input), "\ufeff")
	section := ""
	sections := map[string]bool{}
	fields := map[string]string{}
	allowed := map[string]map[string]bool{
		"interface": {"privatekey": true, "address": true, "dns": true, "mtu": true, "jc": true, "jmin": true, "jmax": true, "s1": true, "s2": true, "s3": true, "s4": true, "h1": true, "h2": true, "h3": true, "h4": true, "i1": true, "i2": true, "i3": true, "i4": true, "i5": true, "headerprotectionkey": true, "contentpaddingaddition": true, "rekeyaftertime": true, "rekeytimeout": true, "rejectaftertime": true, "keepalivetimeout": true, "maxhandshakeattempts": true, "randomtrailers": true, "disablecookies": true},
		"peer":      {"publickey": true, "presharedkey": true, "endpoint": true, "allowedips": true, "persistentkeepalive": true},
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if len(line) > 4096 || strings.ContainsAny(line, "\x00") {
			return bad()
		}
		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				return bad()
			}
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			if allowed[section] == nil || sections[section] {
				return bad()
			}
			sections[section] = true
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if !ok || !allowed[section][key] || value == "" {
			return bad()
		}
		if strings.ContainsAny(value, "#;\"'\\\r\n\t") {
			return bad()
		}
		if section == "interface" {
			switch key {
			case "i1", "i2", "i3", "i4", "i5":
				if !validObfuscationSpec(value) {
					return bad()
				}
			case "jc", "jmin", "jmax", "s1", "s2", "s3", "s4":
				if _, err := strconv.ParseUint(value, 10, 16); err != nil {
					return bad()
				}
			case "h1", "h2", "h3", "h4":
				if !validHeaderRange(value) {
					return bad()
				}
			case "contentpaddingaddition":
				if _, _, ok := parseUint32Range(value); !ok {
					return bad()
				}
			case "rekeyaftertime", "rekeytimeout", "rejectaftertime", "keepalivetimeout", "maxhandshakeattempts":
				if !validTimingRange(value) {
					return bad()
				}
				_, hi, _ := parseUint32Range(value)
				// Engine expiry multiplies this duration by three; handshake
				// counters add two. Reject arithmetic overflow before IPC.
				if key == "rejectaftertime" && hi > 3074457345 || key == "maxhandshakeattempts" && hi > 4294967293 {
					return bad()
				}
			case "randomtrailers", "disablecookies":
				if _, err := strconv.ParseBool(value); err != nil {
					return bad()
				}
			}
		}
		name := section + "." + key
		if _, exists := fields[name]; exists {
			return bad()
		}
		fields[name] = value
	}
	for _, name := range []string{"interface.privatekey", "interface.address", "peer.publickey", "peer.endpoint", "peer.allowedips"} {
		if fields[name] == "" {
			return bad()
		}
	}
	for _, name := range []string{"interface.privatekey", "peer.publickey", "peer.presharedkey"} {
		if value := fields[name]; value != "" {
			b, err := base64.StdEncoding.Strict().DecodeString(value)
			if err != nil || len(b) != 32 {
				return bad()
			}
			clear(b)
		}
	}
	if value := fields["interface.headerprotectionkey"]; value != "" {
		b, err := base64.StdEncoding.Strict().DecodeString(value)
		nonzero := false
		for _, v := range b {
			nonzero = nonzero || v != 0
		}
		clear(b)
		if err != nil || len(b) != 32 || !nonzero {
			return bad()
		}
		for _, name := range []string{"s1", "s2", "s3", "s4"} {
			n, _ := strconv.ParseUint(fields["interface."+name], 10, 16)
			if n < 12 {
				return bad()
			}
		}
	}
	for _, name := range []string{"interface.address", "peer.allowedips"} {
		for _, value := range strings.Split(fields[name], ",") {
			if _, err := netip.ParsePrefix(strings.TrimSpace(value)); err != nil {
				return bad()
			}
		}
	}
	if value := fields["interface.dns"]; value != "" {
		for _, addr := range strings.Split(value, ",") {
			if _, err := netip.ParseAddr(strings.TrimSpace(addr)); err != nil {
				return bad()
			}
		}
	}
	endpoint := fields["peer.endpoint"]
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil || host == "" || len(host) > 253 {
		return bad()
	}
	if _, err := netip.ParseAddr(host); err != nil {
		if strings.HasPrefix(endpoint, "[") || !validEndpointHostname(host) {
			return errors.New("Endpoint должен содержать числовой IP или ASCII-имя DNS; Unicode IDN и специальные символы не поддерживаются")
		}
	}
	n, err := strconv.Atoi(port)
	if err != nil || !decimalDigits(port) || len(port) > 5 || n < 1 || n > 65535 {
		return bad()
	}
	mtu := 1420
	if value := fields["interface.mtu"]; value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 576 || n > 1984 {
			return bad()
		}
		mtu = n
	}
	// Windows buffers in the pinned engine have a 2016-byte datagram limit.
	// Reject incompatible profiles rather than rewriting any server parameters.
	number := func(name string) uint64 { n, _ := strconv.ParseUint(fields["interface."+name], 10, 32); return n }
	if number("s1")+148 > 2016 || number("s2")+92 > 2016 || number("s3")+64 > 2016 || uint64(mtu)+number("s4")+32 > 2016 {
		return bad()
	}
	if value := fields["interface.contentpaddingaddition"]; value != "" {
		_, hi, ok := parseUint32Range(value)
		if !ok || hi > 2016 || uint64(mtu)+number("s4")+32+uint64(hi) > 2016 {
			return bad()
		}
	}
	jc, jmin, jmax := number("jc"), number("jmin"), number("jmax")
	if jc > 128 || jmin > 2016 || jmax > 2016 || jmin > jmax {
		return bad()
	}
	if jc > 0 && (fields["interface.jmin"] == "" || fields["interface.jmax"] == "" || jmin >= jmax) {
		return bad()
	}
	if value := fields["peer.persistentkeepalive"]; value != "" {
		if !validTimingRange(value) {
			return bad()
		}
	}
	return nil
}

// Endpoint bootstrap is the core's only permitted system-resolver exception.
// Import only checks syntax and never resolves the name or contacts a server.
// ASCII punycode labels are preserved as supplied; no IDN conversion is done.
func validEndpointHostname(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range []byte(label) {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

// Grammar and bounds match the pinned amneziawg-go/v3 v3.1.20260828 engine,
// with stricter complete-string parsing and resource limits. The engine's
// obfChain parser otherwise ignores text outside tags and extra tag arguments.
// This storage filter does not replace the core's inactive-device validation.
func validObfuscationSpec(value string) bool {
	if len(value) == 0 || len(value) > 4096 {
		return false
	}
	for _, c := range []byte(value) {
		if c < 0x20 || c > 0x7e {
			return false
		}
	}
	count, fixed := 0, 0
	for remaining := strings.TrimSpace(value); remaining != ""; {
		if remaining[0] != '<' {
			return false
		}
		end := strings.IndexByte(remaining, '>')
		if end < 0 {
			return false
		}
		tag := remaining[1:end]
		if strings.ContainsAny(tag, "<>") {
			return false
		}
		parts := strings.Fields(tag)
		if len(parts) == 0 || len(parts) > 2 {
			return false
		}
		count++
		if count > 32 {
			return false
		}
		switch parts[0] {
		case "t", "d", "ds":
			if len(parts) != 1 {
				return false
			}
			if parts[0] == "t" {
				fixed += 4
			}
		case "b":
			if len(parts) != 2 {
				return false
			}
			hex := strings.TrimPrefix(parts[1], "0x")
			if len(hex) == 0 || len(hex)%2 != 0 {
				return false
			}
			for _, c := range []byte(hex) {
				if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
					return false
				}
			}
			fixed += len(hex) / 2
		case "r", "rc", "rd", "dz":
			if len(parts) != 2 || !decimalDigits(parts[1]) {
				return false
			}
			n, err := strconv.ParseUint(parts[1], 10, 16)
			if err != nil || n > 4096 {
				return false
			}
			if parts[0] == "dz" && (n < 1 || n > 8) {
				return false
			}
			fixed += int(n)
		default:
			return false
		}
		if fixed > 2016 {
			return false
		}
		remaining = strings.TrimSpace(remaining[end+1:])
	}
	return count > 0
}

func decimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, c := range []byte(value) {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func validHeaderRange(value string) bool {
	_, _, ok := parseUint32Range(value)
	return ok
}

func validTimingRange(value string) bool {
	lo, hi, ok := parseUint32Range(value)
	return ok && (lo >= 1 || hi == 0)
}

func parseUint32Range(value string) (uint32, uint32, bool) {
	parts := strings.Split(value, "-")
	if len(parts) < 1 || len(parts) > 2 {
		return 0, 0, false
	}
	var numbers [2]uint64
	for i, part := range parts {
		if !decimalDigits(part) {
			return 0, 0, false
		}
		n, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return 0, 0, false
		}
		numbers[i] = n
	}
	if len(parts) == 1 {
		return uint32(numbers[0]), uint32(numbers[0]), true
	}
	// A full uint32 range overflows the pinned engine's range-length arithmetic.
	return uint32(numbers[0]), uint32(numbers[1]), numbers[0] <= numbers[1] && !(numbers[0] == 0 && numbers[1] == 1<<32-1)
}

// ClientConfigForStorage omits comments, which may contain unrelated secrets.
// It preserves accepted field values and does not normalize AWG parameters.
func ClientConfigForStorage(input []byte) ([]byte, error) {
	if err := ValidateImport(input); err != nil {
		return nil, err
	}
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimPrefix(string(input), "\ufeff"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return []byte(b.String()), nil
}
