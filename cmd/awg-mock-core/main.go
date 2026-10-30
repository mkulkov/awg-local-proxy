// Command awg-mock-core is an independent network-free IPC fixture.
package main

import (
	"github.com/mkulkov/awg-local-proxy/internal/coreclient/mock"
	"os"
)

func main() {
	if err := mock.Serve(os.Stdin, os.Stdout); err != nil {
		os.Exit(1)
	}
}
