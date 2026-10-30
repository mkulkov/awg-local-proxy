// Package mock implements a network-free demonstration core.
package mock

import (
	"bufio"
	"encoding/json"
	"io"

	"github.com/mkulkov/awg-local-proxy/internal/coreclient"
)

const Warning = "Демонстрационный режим — VPN не подключён"

func Serve(in io.Reader, out io.Writer) error {
	r := bufio.NewReaderSize(in, 4096)
	s := coreclient.Status{State: coreclient.Stopped, Demo: true, Message: Warning}
	for {
		m, err := coreclient.ReadMessage(r)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(m.Payload, &payload); err != nil || payload == nil || len(payload) != 0 {
			if err := coreclient.WriteMessage(out, "error", m.ID, map[string]string{"code": "invalid_payload"}); err != nil {
				return err
			}
			continue
		}
		switch m.Type {
		case "hello":
			err = coreclient.WriteMessage(out, "hello", m.ID, coreclient.Hello{ProtocolVersion: 1, CoreVersion: "mock-0.1.0", Commit: "independent", Capabilities: []string{"demo", "status", "disconnect"}})
		case "connect":
			s.State = coreclient.Degraded
			s.Message = Warning + ". Ядро заблокировано лицензией; трафик не передаётся."
			err = coreclient.WriteMessage(out, "status", m.ID, s)
		case "disconnect":
			s.State = coreclient.Stopped
			s.Message = Warning
			err = coreclient.WriteMessage(out, "status", m.ID, s)
		case "get_status":
			err = coreclient.WriteMessage(out, "status", m.ID, s)
		case "validate_config":
			err = coreclient.WriteMessage(out, "error", m.ID, map[string]string{"code": "real_config_not_supported_in_demo"})
		case "shutdown":
			return coreclient.WriteMessage(out, "shutdown", m.ID, struct{}{})
		default:
			err = coreclient.WriteMessage(out, "error", m.ID, map[string]string{"code": "unsupported_command"})
		}
		if err != nil {
			return err
		}
	}
}
