package coreclient

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"
)

const RealCoreVersion = "awg-local-core-0.1.0"

// CoreError exposes only a known stable code, never a child-provided message.
type CoreError struct{ Code string }

func (e *CoreError) Error() string { return "core request failed: " + e.Code }

// TransportError means the verified child's inherited control pipe ended.
// Protocol, identity, hash and configuration errors are deliberately different.
type TransportError struct{}

func (*TransportError) Error() string { return "core control transport unavailable" }

func IsRetryable(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var transport *TransportError
	if errors.As(err, &transport) {
		return true
	}
	var core *CoreError
	return errors.As(err, &core) && core.Code == "connect_failed"
}

func coreResponseError(payload []byte) error {
	var p struct {
		Code string `json:"code"`
	}
	if decodeRealPayload(payload, &p) != nil {
		return &CoreError{Code: "invalid_error_response"}
	}
	switch p.Code {
	case "connect_failed", "invalid_config", "invalid_payload", "invalid_timeout", "already_active", "not_connected", "connection_cancelled", "port_unavailable", "unsupported_command":
		return &CoreError{Code: p.Code}
	default:
		return &CoreError{Code: "unknown_core_error"}
	}
}

func controlPipeError(err error) error {
	var pathError *os.PathError
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, io.ErrShortWrite) || errors.Is(err, os.ErrClosed) || errors.As(err, &pathError) {
		return &TransportError{}
	}
	return errors.New("invalid core control stream")
}

// ConnectRequest contains memory-only configuration. The caller owns Config and
// should clear it after the call. The supervisor clears its encoded byte buffers;
// the Go JSON/runtime implementation cannot guarantee complete memory erasure.
type ConnectRequest struct {
	Config         []byte
	SocksAddress   string
	HTTPAddress    string
	ProbeURL       string
	TimeoutSeconds int
}

// RealSupervisor runs only the explicitly pinned executable. Its Job Object is
// never inherited; closing it (including GUI termination) kills the owned child.
type RealSupervisor struct {
	mu       sync.Mutex
	requests chan struct{}
	cmd      *exec.Cmd
	in       io.WriteCloser
	out      *bufio.Reader
	done     chan struct{}
	job      *realJob
	closed   bool
	seq      uint64
}

func StartReal(executable, expectedSHA256 string) (*RealSupervisor, error) {
	if !filepath.IsAbs(executable) {
		return nil, errors.New("core path must be absolute")
	}
	if err := verifyRealExecutable(executable, expectedSHA256); err != nil {
		return nil, err
	}
	job, err := newRealJob()
	if err != nil {
		return nil, errors.New("cannot establish core process isolation")
	}
	cmd := exec.Command(executable, "--control-stdio")
	configureRealCommand(cmd)
	cmd.Stderr = io.Discard
	in, err := cmd.StdinPipe()
	if err != nil {
		job.close()
		return nil, errors.New("cannot create core control pipe")
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		in.Close()
		job.close()
		return nil, errors.New("cannot create core control pipe")
	}
	if err := cmd.Start(); err != nil {
		in.Close()
		out.Close()
		job.close()
		return nil, errors.New("cannot launch core")
	}
	if err := job.assign(cmd.Process); err != nil {
		in.Close()
		out.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		job.close()
		return nil, errors.New("cannot isolate core process")
	}
	s := &RealSupervisor{requests: make(chan struct{}, 1), cmd: cmd, in: in, out: bufio.NewReader(out), done: make(chan struct{}), job: job}
	go func() { _ = cmd.Wait(); close(s.done) }()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := s.Hello(ctx); err != nil {
		s.stop()
		_ = s.Close()
		return nil, errors.New("core identity or protocol check failed")
	}
	return s, nil
}

func verifyRealExecutable(path, expected string) error {
	digest, err := hex.DecodeString(expected)
	if err != nil || len(digest) != sha256.Size {
		return errors.New("invalid pinned core hash")
	}
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return errors.New("core executable unavailable")
	}
	f, err := os.Open(path)
	if err != nil {
		return errors.New("core executable unavailable")
	}
	defer f.Close()
	const maxBinaryBytes = 256 * 1024 * 1024
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(f, maxBinaryBytes+1))
	if err != nil || n > maxBinaryBytes || !bytes.Equal(h.Sum(nil), digest) {
		return errors.New("core executable hash mismatch")
	}
	return nil
}

func (s *RealSupervisor) request(ctx context.Context, kind string, payload any, expectedType string, target any) error {
	select {
	case s.requests <- struct{}{}:
		defer func() { <-s.requests }()
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return &TransportError{}
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return &TransportError{}
	}
	if err := ctx.Err(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.seq++
	id := strconv.FormatUint(s.seq, 10)
	s.mu.Unlock()
	type result struct {
		m   Message
		err error
	}
	ch := make(chan result, 1)
	go func() {
		if err := writeRealMessage(s.in, kind, id, payload); err != nil {
			ch <- result{err: err}
			return
		}
		m, err := ReadMessage(s.out)
		ch <- result{m, err}
	}()
	select {
	case <-ctx.Done():
		s.stop()
		return ctx.Err()
	case r := <-ch:
		if r.err != nil {
			s.stop()
			return controlPipeError(r.err)
		}
		if r.m.ID != id {
			s.stop()
			return errors.New("invalid core response identifier")
		}
		if r.m.Type == "error" {
			err := coreResponseError(r.m.Payload)
			var ce *CoreError
			if errors.As(err, &ce) && (ce.Code == "unknown_core_error" || ce.Code == "invalid_error_response") {
				s.stop()
			}
			return err
		}
		if r.m.Type != expectedType || decodeRealPayload(r.m.Payload, target) != nil {
			s.stop()
			return errors.New("invalid core response")
		}
		return nil
	}
}

func writeRealMessage(w io.Writer, kind, id string, payload any) error {
	p, err := json.Marshal(payload)
	if err != nil {
		return errors.New("cannot encode core request")
	}
	defer clear(p)
	b, err := json.Marshal(Message{ProtocolVersion: ProtocolVersion, Type: kind, ID: id, Payload: p})
	if err != nil {
		return errors.New("cannot encode core request")
	}
	defer func() { clear(b) }()
	if len(b) > MaxMessageBytes {
		return errors.New("configuration exceeds IPC message limit")
	}
	b = append(b, '\n')
	n, err := w.Write(b)
	if err == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return err
}

func decodeRealPayload(b []byte, target any) error {
	// Reject duplicate top-level payload fields before normal typed decoding.
	d := json.NewDecoder(bytes.NewReader(b))
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return errors.New("invalid payload")
	}
	seen := map[string]bool{}
	allowed := map[string]bool{}
	targetType := reflect.TypeOf(target)
	if targetType == nil || targetType.Kind() != reflect.Pointer || targetType.Elem().Kind() != reflect.Struct {
		return errors.New("invalid response target")
	}
	for i := 0; i < targetType.Elem().NumField(); i++ {
		field := targetType.Elem().Field(i)
		name := field.Tag.Get("json")
		for j, c := range name {
			if c == ',' {
				name = name[:j]
				break
			}
		}
		if name != "" && name != "-" {
			allowed[name] = true
		}
	}
	for d.More() {
		t, err := d.Token()
		if err != nil {
			return err
		}
		key, ok := t.(string)
		if !ok || seen[key] || !allowed[key] {
			return errors.New("invalid payload field")
		}
		seen[key] = true
		var v json.RawMessage
		if err := d.Decode(&v); err != nil {
			return err
		}
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("trailing payload data")
	}
	return nil
}

func (s *RealSupervisor) Hello(ctx context.Context) (Hello, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var h Hello
	if err := s.request(ctx, "hello", struct{}{}, "hello", &h); err != nil {
		return Hello{}, err
	}
	if h.ProtocolVersion != ProtocolVersion || h.CoreVersion != RealCoreVersion || h.Commit != "independent" {
		s.stop()
		return Hello{}, errors.New("unexpected core identity")
	}
	required := map[string]bool{"real_core": false, "stdio": false, "socks5": false, "http": false, "amneziawg": false}
	for _, capability := range h.Capabilities {
		if _, ok := required[capability]; ok {
			required[capability] = true
		}
	}
	for _, present := range required {
		if !present {
			s.stop()
			return Hello{}, errors.New("required core capability missing")
		}
	}
	return h, nil
}

func (s *RealSupervisor) Connect(context.Context) (Status, error) {
	return Status{}, errors.New("select a profile before connecting")
}

func (s *RealSupervisor) ValidateConfig(ctx context.Context, config []byte) error {
	if len(config) == 0 || len(config) > MaxMessageBytes || !utf8.Valid(config) {
		return errors.New("invalid configuration size or encoding")
	}
	quoted := quoteConfig(config)
	defer clear(quoted)
	payload := struct {
		Config json.RawMessage `json:"config"`
	}{quoted}
	if err := writeRealMessage(io.Discard, "validate_config", "18446744073709551615", payload); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var result struct {
		Valid bool `json:"valid"`
	}
	if err := s.request(ctx, "validate_config", payload, "validated", &result); err != nil {
		return err
	}
	if !result.Valid {
		return errors.New("core configuration validation failed")
	}
	return nil
}

func (s *RealSupervisor) ConnectProfile(ctx context.Context, options ConnectRequest) (Status, error) {
	if err := options.validate(); err != nil {
		return Status{}, err
	}
	if options.TimeoutSeconds == 0 {
		options.TimeoutSeconds = 15
	}
	quoted := quoteConfig(options.Config)
	defer clear(quoted)
	payload := struct {
		Config         json.RawMessage `json:"config"`
		SocksAddress   string          `json:"socks_address"`
		HTTPAddress    string          `json:"http_address"`
		ProbeURL       string          `json:"probe_url"`
		TimeoutSeconds int             `json:"timeout_seconds"`
	}{quoted, options.SocksAddress, options.HTTPAddress, options.ProbeURL, options.TimeoutSeconds}
	if err := writeRealMessage(io.Discard, "connect", "18446744073709551615", payload); err != nil {
		return Status{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(options.TimeoutSeconds+5)*time.Second)
	defer cancel()
	status, err := s.statusRequest(ctx, "connect", payload)
	if err == nil && status.State == Connected && (status.SocksAddress != options.SocksAddress || status.HTTPAddress != options.HTTPAddress) {
		s.stop()
		return Status{}, errors.New("core reported unexpected proxy addresses")
	}
	return status, err
}

func (o ConnectRequest) validate() error {
	if len(o.Config) == 0 || len(o.Config) > MaxMessageBytes || !utf8.Valid(o.Config) {
		return errors.New("invalid configuration size or encoding")
	}
	if !realLoopbackAddress(o.SocksAddress) || o.HTTPAddress != "" && !realLoopbackAddress(o.HTTPAddress) || o.SocksAddress == o.HTTPAddress {
		return errors.New("proxy listeners must use distinct IPv4 loopback addresses")
	}
	u, err := url.Parse(o.ProbeURL)
	if len(o.ProbeURL) > 2048 || err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("explicit HTTP or HTTPS tunnel probe is required")
	}
	if o.TimeoutSeconds < 0 || o.TimeoutSeconds > 60 {
		return errors.New("invalid connection timeout")
	}
	return nil
}

func realLoopbackAddress(address string) bool {
	host, port, err := net.SplitHostPort(address)
	n, portErr := strconv.Atoi(port)
	return err == nil && portErr == nil && host == "127.0.0.1" && n > 0 && n <= 65535
}

func quoteConfig(config []byte) []byte {
	capacity := 2
	for _, c := range config {
		switch {
		case c < 0x20:
			capacity += 6
		case c == '"' || c == '\\':
			capacity += 2
		default:
			capacity++
		}
	}
	b := make([]byte, 0, capacity)
	b = append(b, '"')
	const hexChars = "0123456789abcdef"
	for _, c := range config {
		switch c {
		case '"', '\\':
			b = append(b, '\\', c)
		default:
			if c < 0x20 {
				b = append(b, '\\', 'u', '0', '0', hexChars[c>>4], hexChars[c&15])
			} else {
				b = append(b, c)
			}
		}
	}
	return append(b, '"')
}

func (s *RealSupervisor) statusRequest(ctx context.Context, kind string, payload any) (Status, error) {
	var status Status
	if err := s.request(ctx, kind, payload, "status", &status); err != nil {
		return Status{}, err
	}
	if status.Demo || status.Validate() != nil || status.ListenerReady && !realLoopbackAddress(status.SocksAddress) || status.SocksAddress != "" && !realLoopbackAddress(status.SocksAddress) || status.HTTPAddress != "" && !realLoopbackAddress(status.HTTPAddress) {
		s.stop()
		return Status{}, errors.New("unverified core state")
	}
	// Never show raw child-provided text, which could accidentally echo secrets.
	status.Message = safeRealMessage(status.State)
	return status, nil
}

func safeRealMessage(state State) string {
	switch state {
	case Connected:
		return "Туннель подтверждён: handshake и проверка запроса прошли."
	case Connecting, Starting:
		return "Ожидание защищённого подключения."
	case Stopped:
		return "Подключение остановлено."
	default:
		return "Защищённое подключение не подтверждено."
	}
}

func (s *RealSupervisor) Status(ctx context.Context) (Status, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return s.statusRequest(ctx, "get_status", struct{}{})
}
func (s *RealSupervisor) Disconnect(ctx context.Context) (Status, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.statusRequest(ctx, "disconnect", struct{}{})
}
func (s *RealSupervisor) CheckTunnel(ctx context.Context) (Status, error) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	return s.statusRequest(ctx, "check_tunnel", struct{}{})
}

func (s *RealSupervisor) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		_ = s.in.Close()
		_ = s.job.close()
	}
}

func (s *RealSupervisor) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	var response struct{}
	_ = s.request(ctx, "shutdown", struct{}{}, "shutdown", &response)
	cancel()
	s.stop()
	select {
	case <-s.done:
		return nil
	case <-time.After(2 * time.Second):
		_ = s.cmd.Process.Kill()
		select {
		case <-s.done:
			return nil
		case <-time.After(time.Second):
			return errors.New("core process did not terminate")
		}
	}
}

// Kept in one place to prevent accidental fallback to direct proxy transports.
var _ Client = (*RealSupervisor)(nil)
