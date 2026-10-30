package coreclient

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestOnlyKnownTransientErrorsAreRetryable(t *testing.T) {
	for _, tt := range []struct {
		err  error
		want bool
	}{{&TransportError{}, true}, {context.DeadlineExceeded, true}, {context.Canceled, false}, {&CoreError{Code: "connect_failed"}, true}, {&CoreError{Code: "invalid_config"}, false}, {&CoreError{Code: "port_unavailable"}, false}, {&CoreError{Code: "unknown"}, false}, {errors.New("core control pipe failed"), false}} {
		if IsRetryable(tt.err) != tt.want {
			t.Fatalf("wrong retry policy for %T", tt.err)
		}
	}
}
func TestCoreErrorCodesDoNotReflectUnknownInput(t *testing.T) {
	for _, payload := range []string{`{"code":"SYNTHETIC_PRIVATE_SECRET"}`, `{"Code":"connect_failed"}`, `{"code":"connect_failed","extra":"SYNTHETIC_PRIVATE_SECRET"}`, `{"code":"connect_failed","code":"connect_failed"}`} {
		err := coreResponseError([]byte(payload))
		if err == nil || IsRetryable(err) || strings.Contains(err.Error(), "SYNTHETIC_PRIVATE_SECRET") {
			t.Fatal("unknown/malformed child error was exposed or retried")
		}
	}
	if !IsRetryable(coreResponseError([]byte(`{"code":"connect_failed"}`))) {
		t.Fatal("known transient code lost")
	}
	if !IsRetryable(controlPipeError(io.EOF)) || IsRetryable(controlPipeError(errors.New("malformed protocol"))) {
		t.Fatal("protocol failure confused with transport loss")
	}
}
