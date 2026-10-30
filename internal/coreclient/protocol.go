package coreclient

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

type Message struct {
	ProtocolVersion int             `json:"protocol_version"`
	Type            string          `json:"type"`
	ID              string          `json:"id"`
	Payload         json.RawMessage `json:"payload"`
}

// ReadMessage bounds the complete NDJSON line before decoding any payload.
func ReadMessage(r *bufio.Reader) (Message, error) {
	var m Message
	var line []byte
	for {
		part, prefix, err := r.ReadLine()
		if err != nil {
			return m, err
		}
		if len(line)+len(part) > MaxMessageBytes {
			return m, errors.New("IPC message too large")
		}
		line = append(line, part...)
		if !prefix {
			break
		}
	}
	if !utf8.Valid(line) {
		return m, errors.New("invalid IPC encoding")
	}
	check := json.NewDecoder(bytes.NewReader(line))
	start, err := check.Token()
	if err != nil || start != json.Delim('{') {
		return m, errors.New("invalid IPC envelope")
	}
	seen := map[string]bool{}
	for check.More() {
		token, err := check.Token()
		if err != nil {
			return m, errors.New("invalid IPC envelope")
		}
		key, ok := token.(string)
		if !ok || seen[key] {
			return m, errors.New("duplicate IPC field")
		}
		switch key {
		case "protocol_version", "type", "id", "payload":
		default:
			return m, errors.New("unknown IPC field")
		}
		seen[key] = true
		var value json.RawMessage
		if err := check.Decode(&value); err != nil {
			return m, errors.New("invalid IPC envelope")
		}
	}
	d := json.NewDecoder(bytes.NewReader(line))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return m, errors.New("invalid IPC envelope")
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return m, errors.New("trailing IPC data")
	}
	if m.ProtocolVersion != ProtocolVersion || m.ID == "" || len(m.ID) > 64 || m.Type == "" || len(m.Type) > 64 {
		return m, errors.New("unsupported IPC envelope")
	}
	if !json.Valid(m.Payload) {
		return m, errors.New("invalid IPC payload")
	}
	return m, nil
}

func WriteMessage(w io.Writer, kind, id string, payload any) error {
	p, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	m := Message{ProtocolVersion: ProtocolVersion, Type: kind, ID: id, Payload: p}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if len(b) > MaxMessageBytes {
		return errors.New("IPC message too large")
	}
	b = append(b, '\n')
	n, err := w.Write(b)
	if err == nil && n != len(b) {
		return io.ErrShortWrite
	}
	return err
}
