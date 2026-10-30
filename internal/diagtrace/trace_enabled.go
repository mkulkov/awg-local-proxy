//go:build awgtrace

// Package diagtrace provides fixed startup markers in diagnostic builds.
package diagtrace

import (
	"fmt"
	"os"
)

func Step(stage string) { fmt.Fprintln(os.Stderr, "AWG-START:", stage) }
