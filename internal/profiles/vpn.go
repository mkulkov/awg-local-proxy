package profiles

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

// Envelope limits are independent of the smaller native client/IPC budget.
const (
	MaxVPNInputBytes   = 64 * 1024
	MaxVPNDecodedBytes = 256 * 1024
)

type ImportSource string

const (
	ImportSourceConf       ImportSource = "conf"
	ImportSourceAmneziaVPN ImportSource = "amnezia-vpn"
)

// ImportResult contains only the validated native client configuration. Callers
// must clear Config after passing it to encrypted storage. Envelope metadata is
// never returned, even when it contains administrative credentials.
type ImportResult struct {
	Config            []byte
	Source            ImportSource
	DiscardedMetadata bool
}

var errVPNImport = errors.New("экспорт Amnezia не содержит единственный поддерживаемый клиентский AWG-профиль или повреждён; файл не сохранён")

// ImportClientConfig recognizes native .conf and a bounded subset of the
// official Amnezia .vpn/vpn:// export. This is an original format reader, not
// code from amnezia-client. Format evidence: amnezia-client commit
// 7f9b7db482b8b1662b7466937e578008cbcb716e, selfhosted export/import controllers,
// AwgProtocolConfig and ContainerConfig. It does not import server settings.
// No input is written to disk, resolved, logged, or used to start a connection.
func ImportClientConfig(input []byte) (ImportResult, error) {
	if len(input) == 0 || len(input) > MaxVPNInputBytes {
		return ImportResult{}, errVPNImport
	}
	if config, err := ClientConfigForStorage(input); err == nil {
		return ImportResult{Config: config, Source: ImportSourceConf}, nil
	}
	data := input
	if data[0] != 0 && utf8.Valid(data) {
		data = bytes.TrimSpace(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")))
	}
	if bytes.HasPrefix(data, []byte("vpn://")) {
		data = data[len("vpn://"):]
		decoded, err := decodeVPNBase64(data)
		if err != nil {
			return ImportResult{}, errVPNImport
		}
		defer clear(decoded)
		data = decoded
	} else if len(data) > 0 && data[0] != '{' && data[0] != 0 {
		decoded, err := decodeVPNBase64(data)
		if err != nil {
			return ImportResult{}, errVPNImport
		}
		defer clear(decoded)
		data = decoded
	}
	if len(data) > 0 && data[0] != 0 && utf8.Valid(data) {
		data = bytes.TrimSpace(data)
	}
	// qCompress is a big-endian 32-bit uncompressed size followed by zlib.
	// A direct zlib stream is deliberately not guessed as another format.
	if len(data) > 0 && data[0] != '{' {
		decoded, err := decompressVPN(data)
		if err != nil {
			return ImportResult{}, errVPNImport
		}
		defer clear(decoded)
		data = decoded
	}
	root, err := parseVPNObject(data)
	if err != nil {
		return ImportResult{}, errVPNImport
	}
	if version, exists := root["format_version"]; exists && version != json.Number("1") {
		return ImportResult{}, errVPNImport
	}
	containers, ok := root["containers"].([]any)
	if !ok || len(containers) == 0 || len(containers) > 32 {
		return ImportResult{}, errVPNImport
	}
	var selected map[string]any
	for _, value := range containers {
		container, ok := value.(map[string]any)
		if !ok {
			return ImportResult{}, errVPNImport
		}
		name, ok := container["container"].(string)
		if !ok {
			return ImportResult{}, errVPNImport
		}
		_, hasAWG := container["awg"]
		if name != "amnezia-awg" && name != "amnezia-awg2" {
			if hasAWG { // Contradictory protocol identity is not metadata.
				return ImportResult{}, errVPNImport
			}
			continue
		}
		awg, ok := container["awg"].(map[string]any)
		if !ok || selected != nil {
			return ImportResult{}, errVPNImport
		}
		selected = awg
	}
	if selected == nil {
		return ImportResult{}, errVPNImport
	}
	lastConfig, ok := selected["last_config"].(string)
	if !ok || len(lastConfig) > MaxVPNDecodedBytes {
		return ImportResult{}, errVPNImport
	}
	lastConfigBytes := []byte(lastConfig)
	defer clear(lastConfigBytes)
	client, err := parseVPNObject(lastConfigBytes)
	if err != nil {
		return ImportResult{}, errVPNImport
	}
	native, ok := client["config"].(string)
	if !ok || len(native) > MaxImportBytes {
		return ImportResult{}, errVPNImport
	}
	clientBytes := []byte(native)
	defer clear(clientBytes)
	config, err := ClientConfigForStorage(clientBytes)
	if err != nil {
		return ImportResult{}, errVPNImport
	}
	return ImportResult{Config: config, Source: ImportSourceAmneziaVPN, DiscardedMetadata: true}, nil
}

func decodeVPNBase64(input []byte) ([]byte, error) {
	if len(input) == 0 || len(input) > MaxVPNInputBytes {
		return nil, errVPNImport
	}
	// Go's decoder otherwise tolerates embedded newlines. URI escapes, standard
	// base64 alphabet, queries and fragments are outside this known format.
	for _, b := range input {
		if !(b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '-' || b == '_' || b == '=') {
			return nil, errVPNImport
		}
	}
	encoding := base64.RawURLEncoding.Strict()
	if bytes.ContainsRune(input, '=') {
		encoding = base64.URLEncoding.Strict()
	}
	output := make([]byte, encoding.DecodedLen(len(input)))
	n, err := encoding.Decode(output, input)
	if err != nil {
		clear(output)
		return nil, errVPNImport
	}
	return output[:n], nil
}

func decompressVPN(input []byte) ([]byte, error) {
	if len(input) < 6 {
		return nil, errVPNImport
	}
	size := binary.BigEndian.Uint32(input[:4])
	if size == 0 || size > MaxVPNDecodedBytes {
		return nil, errVPNImport
	}
	compressed := bytes.NewReader(input[4:]) // ByteReader prevents read-ahead.
	r, err := zlib.NewReader(compressed)
	if err != nil {
		return nil, errVPNImport
	}
	output, readErr := io.ReadAll(io.LimitReader(r, int64(size)+1))
	closeErr := r.Close()
	if readErr != nil || closeErr != nil || len(output) != int(size) || compressed.Len() != 0 {
		clear(output)
		return nil, errVPNImport
	}
	return output, nil
}

// Parse every object, including ignored metadata, without duplicate keys or
// unbounded recursion. The result is used only transiently in this call.
func parseVPNObject(input []byte) (map[string]any, error) {
	if len(input) == 0 || len(input) > MaxVPNDecodedBytes || !utf8.Valid(input) {
		return nil, errVPNImport
	}
	d := json.NewDecoder(bytes.NewReader(input))
	d.UseNumber()
	budget := 16384
	value, err := readVPNJSON(d, 0, &budget)
	if err != nil {
		return nil, errVPNImport
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, errVPNImport
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, errVPNImport
	}
	return object, nil
}

func readVPNJSON(d *json.Decoder, depth int, budget *int) (any, error) {
	*budget -= 1
	if depth > 16 || *budget < 0 {
		return nil, errVPNImport
	}
	token, err := d.Token()
	if err != nil {
		return nil, errVPNImport
	}
	switch token {
	case json.Delim('{'):
		object := make(map[string]any)
		seen := make(map[string]bool)
		for d.More() {
			keyToken, err := d.Token()
			key, ok := keyToken.(string)
			if err != nil || !ok || len(key) > 128 || seen[strings.ToLower(key)] {
				return nil, errVPNImport
			}
			seen[strings.ToLower(key)] = true
			value, err := readVPNJSON(d, depth+1, budget)
			if err != nil {
				return nil, errVPNImport
			}
			object[key] = value
		}
		if end, err := d.Token(); err != nil || end != json.Delim('}') {
			return nil, errVPNImport
		}
		return object, nil
	case json.Delim('['):
		var array []any
		for d.More() {
			value, err := readVPNJSON(d, depth+1, budget)
			if err != nil {
				return nil, errVPNImport
			}
			array = append(array, value)
		}
		if end, err := d.Token(); err != nil || end != json.Delim(']') {
			return nil, errVPNImport
		}
		return array, nil
	default:
		if _, delim := token.(json.Delim); delim {
			return nil, errVPNImport
		}
		return token, nil
	}
}
