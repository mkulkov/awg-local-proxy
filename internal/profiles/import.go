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

// ValidateImport is a conservative original client-only storage filter. It does
// not assert cryptographic validity or engine compatibility and never executes a
// hook. Configurations remain sealed and are not sent to a core in this demo.
func ValidateImport(input []byte) error {
	bad := func() error {
		return errors.New("конфигурация не входит в поддерживаемый безопасный поднабор .conf; файл не сохранён")
	}
	if len(input) == 0 || len(input) > maxSecretBytes || !utf8.Valid(input) {
		return bad()
	}
	text := strings.TrimPrefix(string(input), "\ufeff")
	section := ""
	sections := map[string]bool{}
	fields := map[string]string{}
	allowed := map[string]map[string]bool{
		"interface": {"privatekey": true, "address": true, "dns": true, "mtu": true, "jc": true, "jmin": true, "jmax": true, "s1": true, "s2": true, "s3": true, "s4": true, "h1": true, "h2": true, "h3": true, "h4": true, "i1": true, "i2": true, "i3": true, "i4": true, "i5": true},
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
				return errors.New("поля I1–I5 пока не импортируются: требуется проверенная грамматика закреплённой версии AWG")
			case "jc", "jmin", "jmax", "s1", "s2", "s3", "s4":
				if _, err := strconv.ParseUint(value, 10, 16); err != nil {
					return bad()
				}
			case "h1", "h2", "h3", "h4":
				if _, err := strconv.ParseUint(value, 10, 32); err != nil {
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
	host, port, err := net.SplitHostPort(fields["peer.endpoint"])
	if err != nil {
		return bad()
	}
	if _, err := netip.ParseAddr(host); err != nil {
		return errors.New("демонстрационный импорт принимает только числовой IP Endpoint; имя сервера не сохранено")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return bad()
	}
	if value := fields["interface.mtu"]; value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 576 || n > 9000 {
			return bad()
		}
	}
	if value := fields["peer.persistentkeepalive"]; value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 || n > 65535 {
			return bad()
		}
	}
	return nil
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
