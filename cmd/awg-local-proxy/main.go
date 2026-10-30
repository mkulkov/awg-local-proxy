package main

import (
	"context"
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
	"github.com/mkulkov/awg-local-proxy/internal/diagtrace"
	"github.com/mkulkov/awg-local-proxy/internal/profiles"
	"github.com/mkulkov/awg-local-proxy/internal/session"
	"github.com/mkulkov/awg-local-proxy/internal/ui"
)

// Set by the local build when a reviewed core binary is bundled. An empty
// value deliberately keeps the network-free demonstration mode.
var expectedRealCoreSHA256 string

func main() {
	diagtrace.Step("main entered")
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
	diagtrace.Step("instance check begin")
	instance, existing, err := application.AcquireInstance()
	diagtrace.Step("instance check returned")
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot establish application instance")
		showStartupError("Не удалось запустить приложение: невозможно проверить учётную запись или создать объект запуска. Подключение не выполнялось.")
		return
	}
	if existing {
		return
	}
	defer instance.Close()
	diagtrace.Step("fyne application begin")
	a := app.NewWithID("io.github.mkulkov.awg-local-proxy.demo")
	diagtrace.Step("fyne application returned")
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "executable unavailable")
		showStartupError("Не удалось определить расположение приложения. Подключение не выполнялось.")
		return
	}
	var core coreclient.Client
	var real *coreclient.RealSupervisor
	diagtrace.Step("core startup begin")
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
	diagtrace.Step("core startup returned")
	if err != nil {
		w := a.NewWindow("Ошибка")
		dialog.ShowInformation("Ядро недоступно", "Не удалось проверить или запустить компонент. Подключение не выполнено.", w)
		w.ShowAndRun()
		return
	}
	diagtrace.Step("profile store begin")
	root, storageErr := profiles.DefaultRoot()
	var store *profiles.Store
	if storageErr == nil {
		store, storageErr = profiles.Open(root, profiles.DPAPI{})
	}
	diagtrace.Step("profile store returned")
	var managed *session.Controller
	if real != nil {
		factory := func(ctx context.Context) (session.Backend, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			child, err := coreclient.StartReal(filepath.Join(filepath.Dir(exe), "awg-proxy-core.exe"), expectedRealCoreSHA256)
			// The controller owns even a child created after cancellation and
			// retains it until shutdown is positively confirmed.
			return child, err
		}
		load := func(id string) ([]byte, error) {
			if store == nil {
				return nil, fmt.Errorf("profile storage unavailable")
			}
			return store.ReadSecret(id)
		}
		managed = session.New(factory, load, real)
		core = managed
	}
	defer core.Close()
	if managed != nil {
		diagtrace.Step("power registration begin")
		power, powerErr := application.WatchPowerEvents(func(event application.PowerEvent) {
			switch event {
			case application.PowerSuspend:
				_, _ = managed.Suspend()
			case application.PowerResume:
				_, _ = managed.Resume()
			}
		})
		diagtrace.Step("power registration returned")
		if powerErr != nil {
			w := a.NewWindow("Ошибка")
			dialog.ShowInformation("Управление питанием недоступно", "Не удалось зарегистрировать обработку сна и возобновления. Подключение не запускается.", w)
			w.ShowAndRun()
			return
		}
		// Stop native event delivery before the controller is closed on exit.
		defer power.Close()
	}
	diagtrace.Step("ui construction begin")
	w := ui.New(a, core, managed, store, storageErr)
	diagtrace.Step("ui construction returned")
	instance.Watch(func() { fyne.Do(func() { w.Show(); w.RequestFocus() }) })
	diagtrace.Step("show and run begin")
	w.ShowAndRun()
	diagtrace.Step("show and run returned")
}
