package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"

	application "github.com/mkulkov/awg-local-proxy/internal/app"
	"github.com/mkulkov/awg-local-proxy/internal/coreclient"
	"github.com/mkulkov/awg-local-proxy/internal/coreclient/mock"
	"github.com/mkulkov/awg-local-proxy/internal/profiles"
	"github.com/mkulkov/awg-local-proxy/internal/ui"
)

// Set by the local build when a reviewed core binary is bundled. An empty
// value deliberately keeps the network-free demonstration mode.
var expectedRealCoreSHA256 string

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--mock-core" {
		if err := mock.Serve(os.Stdin, os.Stdout); err != nil {
			os.Exit(1)
		}
		return
	}
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "unsupported arguments")
		os.Exit(2)
	}
	instance, existing, err := application.AcquireInstance()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot establish application instance")
		return
	}
	if existing {
		return
	}
	defer instance.Close()
	a := app.NewWithID("io.github.mkulkov.awg-local-proxy.demo")
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "executable unavailable")
		return
	}
	var core coreclient.Client
	var real *coreclient.RealSupervisor
	if expectedRealCoreSHA256 != "" {
		real, err = coreclient.StartReal(filepath.Join(filepath.Dir(exe), "awg-proxy-core.exe"), expectedRealCoreSHA256)
		core = real
	} else {
		var b []byte
		b, err = os.ReadFile(exe)
		if err == nil {
			h := sha256.Sum256(b)
			core, err = coreclient.StartMock(exe, hex.EncodeToString(h[:]))
		}
	}
	if err != nil {
		w := a.NewWindow("Ошибка")
		dialog.ShowInformation("Ядро недоступно", "Не удалось проверить или запустить компонент. Подключение не выполнено.", w)
		w.ShowAndRun()
		return
	}
	defer core.Close()
	root, storageErr := profiles.DefaultRoot()
	var store *profiles.Store
	if storageErr == nil {
		store, storageErr = profiles.Open(root, profiles.DPAPI{})
	}
	w := ui.New(a, core, real, store, storageErr)
	instance.Watch(func() { fyne.Do(func() { w.Show(); w.RequestFocus() }) })
	w.ShowAndRun()
}
