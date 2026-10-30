package coreclient

import (
	"context"
	"errors"
	"time"
)

const ProtocolVersion = 1
const MaxMessageBytes = 64 * 1024

type State string

const (
	Stopped      State = "Stopped"
	Starting     State = "Starting"
	Connecting   State = "Connecting"
	Connected    State = "Connected"
	Degraded     State = "Degraded"
	Reconnecting State = "Reconnecting"
	Stopping     State = "Stopping"
	Error        State = "Error"
)

type Status struct {
	State          State      `json:"state"`
	Demo           bool       `json:"demo"`
	ListenerReady  bool       `json:"listener_ready"`
	TunnelVerified bool       `json:"tunnel_verified"`
	HandshakeAt    *time.Time `json:"handshake_at,omitempty"`
	Message        string     `json:"message"`
	SocksAddress   string     `json:"socks_address,omitempty"`
	HTTPAddress    string     `json:"http_address,omitempty"`
}

type Hello struct {
	ProtocolVersion int      `json:"protocol_version"`
	CoreVersion     string   `json:"core_version"`
	Commit          string   `json:"commit"`
	Capabilities    []string `json:"capabilities"`
}

type Client interface {
	Hello(context.Context) (Hello, error)
	Connect(context.Context) (Status, error)
	Disconnect(context.Context) (Status, error)
	Status(context.Context) (Status, error)
	Close() error
}

func (s Status) Validate() error {
	switch s.State {
	case Stopped, Starting, Connecting, Connected, Degraded, Reconnecting, Stopping, Error:
	default:
		return errors.New("invalid core state")
	}
	if s.Demo && (s.State == Connected || s.ListenerReady || s.TunnelVerified || s.HandshakeAt != nil) {
		return errors.New("demo core cannot claim a listener or tunnel")
	}
	if s.State == Connected && (!s.ListenerReady || !s.TunnelVerified || s.HandshakeAt == nil) {
		return errors.New("connected requires listener, handshake and tunnel probe")
	}
	return nil
}
