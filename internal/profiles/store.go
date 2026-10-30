package profiles

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const maxSecretBytes = 64 * 1024

var validID = regexp.MustCompile(`^[a-f0-9]{32}$`)

type Metadata struct {
	Schema    int       `json:"schema"`
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Demo      bool      `json:"demo"`
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
	p := os.Getenv("LOCALAPPDATA")
	if p == "" || !filepath.IsAbs(p) {
		return "", errors.New("LOCALAPPDATA unavailable")
	}
	return filepath.Join(p, "AwgLocalProxy", "profiles"), nil
}

func Open(root string, protector Protector) (*Store, error) {
	if !filepath.IsAbs(root) || protector == nil {
		return nil, errors.New("invalid profile storage")
	}
	// Refuse symlink/reparse ancestors before touching user data.
	for p := root; ; p = filepath.Dir(p) {
		if fi, err := os.Lstat(p); err == nil && fi.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("profile storage cannot use symlinks")
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	if err := restrict(root); err != nil {
		return nil, err
	}
	return &Store{root: root, protector: protector}, nil
}

func validateName(name string) error {
	if strings.TrimSpace(name) == "" || len([]rune(name)) > 80 || strings.ContainsAny(name, "\r\n\x00") {
		return errors.New("имя профиля должно содержать от 1 до 80 символов")
	}
	return nil
}

// CreateDemo stores only metadata. No VPN configuration is accepted by the demo UI.
func (s *Store) CreateDemo(name string) (Metadata, error) { return s.create(name, nil, true) }

// CreateEncrypted is an independent storage boundary, not a configuration parser.
func (s *Store) CreateEncrypted(name string, secret []byte) (Metadata, error) {
	if len(secret) == 0 || len(secret) > maxSecretBytes {
		return Metadata{}, errors.New("invalid profile size")
	}
	return s.create(name, secret, false)
}

func (s *Store) create(name string, secret []byte, demo bool) (Metadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateName(name); err != nil {
		return Metadata{}, err
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return Metadata{}, err
	}
	m := Metadata{Schema: 1, ID: hex.EncodeToString(id[:]), Name: strings.TrimSpace(name), CreatedAt: time.Now().UTC(), Demo: demo}
	if !demo {
		b, err := s.protector.Protect(secret)
		if err != nil {
			return Metadata{}, errors.New("profile encryption failed")
		}
		if err := atomicWrite(s.root, m.ID+".dpapi", b); err != nil {
			return Metadata{}, err
		}
	}
	b, _ := json.Marshal(m)
	if err := atomicWrite(s.root, m.ID+".json", b); err != nil {
		return Metadata{}, err
	}
	return m, nil
}

func (s *Store) List() ([]Metadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	result := []Metadata{}
	for _, item := range items {
		if item.IsDir() || filepath.Ext(item.Name()) != ".json" {
			continue
		}
		if !validID.MatchString(strings.TrimSuffix(item.Name(), ".json")) {
			continue
		}
		b, err := readBounded(filepath.Join(s.root, item.Name()), 4096)
		if err != nil {
			return nil, errors.New("profile metadata unreadable")
		}
		var m Metadata
		if json.Unmarshal(b, &m) != nil || m.Schema != 1 || m.ID+".json" != item.Name() || validateName(m.Name) != nil {
			return nil, errors.New("profile metadata invalid; no automatic migration")
		}
		result = append(result, m)
	}
	return result, nil
}

func (s *Store) Rename(id, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validID.MatchString(id) {
		return errors.New("invalid profile ID")
	}
	if err := validateName(name); err != nil {
		return err
	}
	b, err := readBounded(filepath.Join(s.root, id+".json"), 4096)
	if err != nil {
		return err
	}
	var m Metadata
	if json.Unmarshal(b, &m) != nil || m.Schema != 1 || m.ID != id {
		return errors.New("invalid profile metadata")
	}
	m.Name = strings.TrimSpace(name)
	b, _ = json.Marshal(m)
	return atomicWrite(s.root, id+".json", b)
}

func (s *Store) ReadSecret(id string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validID.MatchString(id) {
		return nil, errors.New("invalid profile ID")
	}
	b, err := readBounded(filepath.Join(s.root, id+".dpapi"), maxSecretBytes+4096)
	if err != nil {
		return nil, errors.New("encrypted profile unavailable")
	}
	p, err := s.protector.Unprotect(b)
	if err != nil {
		return nil, errors.New("profile decryption failed")
	}
	if len(p) > maxSecretBytes {
		clear(p)
		return nil, errors.New("decrypted profile too large")
	}
	return p, nil
}

// Delete is only called after an explicit UI confirmation and while stopped.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validID.MatchString(id) {
		return errors.New("invalid profile ID")
	}
	for _, ext := range []string{".dpapi", ".json"} {
		path := filepath.Join(s.root, id+ext)
		if fi, err := os.Lstat(path); err == nil {
			if !fi.Mode().IsRegular() {
				return errors.New("invalid profile file")
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	for _, ext := range []string{".dpapi", ".json"} {
		if err := os.Remove(filepath.Join(s.root, id+ext)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func readBounded(path string, max int64) ([]byte, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() || fi.Size() > max {
		return nil, errors.New("invalid profile file")
	}
	return os.ReadFile(path)
}

func atomicWrite(root, name string, b []byte) error {
	f, err := os.CreateTemp(root, ".pending-")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := restrict(tmp); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp, filepath.Join(root, name))
}
