// Package preferences stores opt-in startup choices without plaintext files.
// Loading settings never starts a process or sends network traffic.
package preferences

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/mkulkov/awg-local-proxy/internal/profiles"
)

const (
	maxPlaintextBytes  = 8 * 1024
	maxCiphertextBytes = maxPlaintextBytes + 4096
	filename           = "autoconnect.dpapi"
)

var (
	ErrUnavailable        = errors.New("protected preferences unavailable")
	ErrInvalid            = errors.New("invalid protected preferences")
	ErrProfileUnavailable = errors.New("saved auto-connect profile unavailable")
	profileID             = regexp.MustCompile(`^[a-f0-9]{32}$`)
)

// Settings contains no VPN key. ProbeURL may contain a private token, so the
// complete object (including flags) is encrypted at rest. AutoReconnect is an
// independent choice; it does not enable AutoConnect or authorize a new URL.
type Settings struct {
	AutoConnect   bool   `json:"auto_connect"`
	AutoReconnect bool   `json:"auto_reconnect"`
	ProfileID     string `json:"profile_id"`
	ProbeURL      string `json:"probe_url"`
	SocksPort     int    `json:"socks_port"`
	HTTPPort      int    `json:"http_port"`
	HTTPEnabled   bool   `json:"http_enabled"`
}

type Protector interface {
	Protect([]byte) ([]byte, error)
	Unprotect([]byte) ([]byte, error)
}

type Store struct {
	mu        sync.Mutex
	root      string
	protector Protector
}

func DefaultRoot() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" || !filepath.IsAbs(base) {
		return "", ErrUnavailable
	}
	return filepath.Join(base, "AwgLocalProxy", "preferences"), nil
}

func OpenDefault() (*Store, error) {
	root, err := DefaultRoot()
	if err != nil {
		return nil, err
	}
	return Open(root, profiles.DPAPI{})
}

// Open permits an injected protector for isolated tests. Production callers
// should use OpenDefault, which uses Windows DPAPI CurrentUser.
func Open(root string, protector Protector) (*Store, error) {
	if !filepath.IsAbs(root) || protector == nil {
		return nil, ErrUnavailable
	}
	root = filepath.Clean(root)
	if !safeAncestors(root) {
		return nil, ErrUnavailable
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, ErrUnavailable
	}
	if err := restrict(root); err != nil {
		return nil, ErrUnavailable
	}
	return &Store{root: root, protector: protector}, nil
}

func (v Settings) Validate() error {
	if !v.AutoConnect {
		if v.ProfileID != "" || v.ProbeURL != "" || v.SocksPort != 0 || v.HTTPPort != 0 || v.HTTPEnabled {
			return ErrInvalid
		}
		return nil
	}
	if !profileID.MatchString(v.ProfileID) || len(v.ProbeURL) == 0 || len(v.ProbeURL) > 2048 || strings.ContainsAny(v.ProbeURL, "\x00\r\n\t") {
		return ErrInvalid
	}
	u, err := url.Parse(v.ProbeURL)
	if err != nil || u.Opaque != "" || u.User != nil || u.Fragment != "" || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return ErrInvalid
	}
	if strings.ContainsAny(u.Hostname(), " \t\r\n%/\\") {
		return ErrInvalid
	}
	if strings.HasSuffix(u.Host, ":") {
		return ErrInvalid
	}
	if p := u.Port(); p != "" {
		n, e := strconv.Atoi(p)
		if e != nil || n < 1 || n > 65535 {
			return ErrInvalid
		}
	}
	if v.SocksPort < 1 || v.SocksPort > 65535 || v.HTTPPort < 1 || v.HTTPPort > 65535 || v.HTTPEnabled && v.SocksPort == v.HTTPPort {
		return ErrInvalid
	}
	return nil
}

// Load returns the disabled zero value when absent. Every other failure also
// returns the disabled zero value, together with a safe error without input.
func (s *Store) Load() (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

// LoadForAutoConnect checks exactly the saved profile before an application
// may use startup settings. The callback must verify existence, non-demo state
// and decryptability. A failed check never substitutes another profile.
func (s *Store) LoadForAutoConnect(checkProfile func(string) error) (Settings, error) {
	v, err := s.Load()
	if err != nil {
		return Settings{}, err
	}
	if v.AutoConnect && (checkProfile == nil || checkProfile(v.ProfileID) != nil) {
		return Settings{}, ErrProfileUnavailable
	}
	return v, nil
}

func (s *Store) load() (Settings, error) {
	if !safeAncestors(s.root) {
		return Settings{}, ErrUnavailable
	}
	path := filepath.Join(s.root, filename)
	fi, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return Settings{}, nil
	}
	if err != nil || !fi.Mode().IsRegular() || reparse(path) || fi.Size() > maxCiphertextBytes {
		return Settings{}, ErrUnavailable
	}
	f, err := os.Open(path)
	if err != nil {
		return Settings{}, ErrUnavailable
	}
	defer f.Close()
	actual, err := f.Stat()
	if err != nil || !actual.Mode().IsRegular() || !os.SameFile(fi, actual) {
		return Settings{}, ErrUnavailable
	}
	cipher, err := io.ReadAll(io.LimitReader(f, maxCiphertextBytes+1))
	defer clear(cipher)
	if err != nil || len(cipher) == 0 || len(cipher) > maxCiphertextBytes {
		return Settings{}, ErrUnavailable
	}
	plain, err := s.protector.Unprotect(cipher)
	defer clear(plain)
	if err != nil || len(plain) == 0 || len(plain) > maxPlaintextBytes {
		return Settings{}, ErrUnavailable
	}
	v, err := decode(plain)
	if err != nil {
		return Settings{}, ErrInvalid
	}
	return v, nil
}

func (s *Store) Save(v Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v.Validate() != nil {
		return ErrInvalid
	}
	if !safeAncestors(s.root) {
		return ErrUnavailable
	}
	plain, err := json.Marshal(struct {
		Schema   int      `json:"schema"`
		Settings Settings `json:"settings"`
	}{1, v})
	defer clear(plain)
	if err != nil || len(plain) > maxPlaintextBytes {
		return ErrInvalid
	}
	cipher, err := s.protector.Protect(plain)
	defer clear(cipher)
	if err != nil || len(cipher) == 0 || len(cipher) > maxCiphertextBytes {
		return ErrUnavailable
	}
	if err := atomicWrite(s.root, cipher); err != nil {
		return ErrUnavailable
	}
	return nil
}

func decode(b []byte) (Settings, error) {
	if !utf8.Valid(b) {
		return Settings{}, ErrInvalid
	}
	outer, err := object(b, map[string]bool{"schema": true, "settings": true})
	if err != nil || len(outer) != 2 {
		return Settings{}, ErrInvalid
	}
	var schema int
	if json.Unmarshal(outer["schema"], &schema) != nil || schema != 1 {
		return Settings{}, ErrInvalid
	}
	fields, err := object(outer["settings"], map[string]bool{"auto_connect": true, "auto_reconnect": true, "profile_id": true, "probe_url": true, "socks_port": true, "http_port": true, "http_enabled": true})
	if err != nil || len(fields) != 7 {
		return Settings{}, ErrInvalid
	}
	var v Settings
	if json.Unmarshal(outer["settings"], &v) != nil || v.Validate() != nil {
		return Settings{}, ErrInvalid
	}
	return v, nil
}

func object(b []byte, allowed map[string]bool) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(b))
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return nil, ErrInvalid
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		t, err = d.Token()
		if err != nil {
			return nil, ErrInvalid
		}
		key, ok := t.(string)
		if !ok || !allowed[key] {
			return nil, ErrInvalid
		}
		if _, exists := fields[key]; exists {
			return nil, ErrInvalid
		}
		var value json.RawMessage
		if d.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, ErrInvalid
		}
		fields[key] = value
	}
	if t, err = d.Token(); err != nil || t != json.Delim('}') {
		return nil, ErrInvalid
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, ErrInvalid
	}
	return fields, nil
}

func safeAncestors(path string) bool {
	for p := path; ; p = filepath.Dir(p) {
		if fi, err := os.Lstat(p); err == nil {
			if !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 || reparse(p) {
				return false
			}
		} else if !os.IsNotExist(err) {
			return false
		}
		if filepath.Dir(p) == p {
			return true
		}
	}
}

func atomicWrite(root string, cipher []byte) error {
	path := filepath.Join(root, filename)
	if fi, err := os.Lstat(path); err == nil {
		if !fi.Mode().IsRegular() || reparse(path) {
			return ErrUnavailable
		}
	} else if !os.IsNotExist(err) {
		return ErrUnavailable
	}
	f, err := os.CreateTemp(root, ".preferences-")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = restrict(tmp); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(cipher); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp, path)
}
