package ui

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/mkulkov/awg-local-proxy/internal/coreclient"
	"github.com/mkulkov/awg-local-proxy/internal/profiles"
)

func New(a fyne.App, core coreclient.Client, store *profiles.Store, storageError error) fyne.Window {
	w := a.NewWindow(appTitle)
	w.Resize(fyne.NewSize(700, 540))
	w.SetMaster()
	status := widget.NewLabel("Остановлено")
	detail := widget.NewLabel(demoWarning)
	detail.Wrapping = fyne.TextWrapWord
	profile := widget.NewSelect([]string{"Демонстрационный профиль"}, nil)
	profile.SetSelectedIndex(0)
	var loaded []profiles.Metadata
	refreshProfiles := func() {
		if store == nil {
			return
		}
		items, err := store.List()
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		loaded = items
		names := []string{"Демонстрационный профиль"}
		for _, m := range items {
			names = append(names, m.Name)
		}
		profile.SetOptions(names)
		profile.SetSelectedIndex(0)
	}
	refreshProfiles()
	active := false
	var primary *widget.Button
	run := func(connect bool) {
		primary.Disable()
		profile.Disable()
		status.SetText("Обмен с демонстрационным процессом…")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			var s coreclient.Status
			var err error
			if connect {
				s, err = core.Connect(ctx)
			} else {
				s, err = core.Disconnect(ctx)
			}
			fyne.Do(func() {
				primary.Enable()
				profile.Enable()
				if err != nil {
					status.SetText("Ошибка демонстрационного процесса")
					detail.SetText("Процесс недоступен. Закройте и снова откройте приложение.")
					return
				}
				active = connect
				if connect {
					primary.SetText(disconnectLabel)
					status.SetText("Демонстрация активна · туннеля нет")
				} else {
					primary.SetText(connectLabel)
					status.SetText("Остановлено")
				}
				detail.SetText(s.Message)
			})
		}()
	}
	primary = widget.NewButtonWithIcon(connectLabel, theme.MediaPlayIcon(), func() { run(!active) })
	primary.Importance = widget.HighImportance
	address := func(label, value string) fyne.CanvasObject {
		entry := widget.NewEntry()
		entry.SetText(value)
		entry.Disable()
		copy := widget.NewButtonWithIcon("Копировать", theme.ContentCopyIcon(), func() { w.Clipboard().SetContent(value) })
		copy.Disable() // No listener exists in this build; avoid suggesting it is usable.
		return container.NewBorder(nil, nil, widget.NewLabel(label), copy, entry)
	}
	connection := container.NewVBox(widget.NewLabelWithStyle(demoWarning, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), widget.NewSeparator(), widget.NewLabel("Профиль"), profile, status, primary, detail, widget.NewSeparator(), address("SOCKS5", "127.0.0.1:1080"), address("HTTP (выключен)", "127.0.0.1:8080"), widget.NewLabel("Адреса запланированы. В демонстрации порты не открыты."))
	name := widget.NewEntry()
	name.SetPlaceHolder("Название демонстрационного профиля")
	create := widget.NewButton("Добавить демонстрационный профиль", func() {
		if store == nil {
			return
		}
		_, err := store.CreateDemo(name.Text)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		name.SetText("")
		refreshProfiles()
	})
	rename := widget.NewButton("Переименовать выбранный профиль", func() {
		i := profile.SelectedIndex() - 1
		if store == nil || i < 0 || i >= len(loaded) {
			dialog.ShowInformation("Профиль", "Выберите сохранённый профиль на вкладке «Подключение».", w)
			return
		}
		e := widget.NewEntry()
		id, originalName := loaded[i].ID, loaded[i].Name
		e.SetText(originalName)
		dialog.ShowForm("Переименовать профиль", "Сохранить", "Отмена", []*widget.FormItem{widget.NewFormItem("Имя", e)}, func(ok bool) {
			if ok {
				if err := store.Rename(id, e.Text); err != nil {
					dialog.ShowError(err, w)
				} else {
					refreshProfiles()
				}
			}
		}, w)
	})
	profileInfo := widget.NewLabel("Можно сохранить demo-профиль или ограниченный клиентский .conf в DPAPI. Импорт не запускает VPN. Один Peer, числовой Endpoint, только числовые J/S/H. Поля I1–I5, диапазоны H, .vpn и неизвестные поля пока не поддержаны.")
	profileInfo.Wrapping = fyne.TextWrapWord
	var importButton *widget.Button
	importButton = widget.NewButton("Импортировать .conf в DPAPI (без подключения)", func() {
		if store == nil {
			return
		}
		d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, openErr error) {
			if openErr != nil {
				dialog.ShowInformation("Импорт", "Не удалось открыть файл.", w)
				return
			}
			if reader == nil {
				return
			}
			filename := reader.URI().Name()
			if filepath.Ext(filename) != ".conf" {
				reader.Close()
				dialog.ShowInformation("Импорт", "Поддерживается только клиентский .conf.", w)
				return
			}
			importButton.Disable()
			go func() {
				defer reader.Close()
				userError := "Не удалось прочитать или зашифровать файл. Профиль не сохранён."
				data, err := io.ReadAll(io.LimitReader(reader, 65537))
				defer clear(data)
				if err == nil && len(data) > 65536 {
					err = errors.New("файл превышает 64 КиБ")
				}
				if err == nil {
					var client []byte
					client, err = profiles.ClientConfigForStorage(data)
					if err != nil {
						userError = err.Error()
					}
					defer clear(client)
					if err == nil {
						_, err = store.CreateEncrypted(filename, client)
					}
				}
				fyne.Do(func() {
					importButton.Enable()
					if err != nil {
						dialog.ShowInformation("Импорт не выполнен", userError, w)
					} else {
						refreshProfiles()
						dialog.ShowInformation("Профиль сохранён", "Конфигурация зашифрована DPAPI. VPN не подключён; проверка совместимости с ядром ещё не выполнялась.", w)
					}
				})
			}()
		}, w)
		d.SetFilter(storage.NewExtensionFileFilter([]string{".conf"}))
		d.Show()
	})
	deleteButton := widget.NewButton("Удалить выбранный сохранённый профиль", func() {
		i := profile.SelectedIndex() - 1
		if store == nil || i < 0 || i >= len(loaded) {
			dialog.ShowInformation("Профиль", "Выберите сохранённый профиль на вкладке «Подключение».", w)
			return
		}
		if active {
			dialog.ShowInformation("Сначала остановите демонстрацию", "Остановите демонстрацию перед удалением профиля.", w)
			return
		}
		id := loaded[i].ID
		dialog.ShowConfirm("Удалить профиль?", "Будут удалены метаданные и зашифрованная копия. Исходный импортированный файл останется.", func(ok bool) {
			if ok {
				if err := store.Delete(id); err != nil {
					dialog.ShowInformation("Ошибка", "Не удалось удалить профиль.", w)
				} else {
					refreshProfiles()
				}
			}
		}, w)
	})
	if storageError != nil {
		profileInfo.SetText("Хранилище недоступно. Профили не будут сохраняться.")
		create.Disable()
		rename.Disable()
		importButton.Disable()
		deleteButton.Disable()
	}
	profilePage := container.NewVBox(profileInfo, name, create, importButton, rename, deleteButton)
	apps := widget.NewRichTextFromMarkdown("### Настройка приложений\nРабочий прокси пока отсутствует. Не меняйте настройки своих приложений для этой демонстрации.\n\n* **Браузер:** будущий отдельный профиль с SOCKS5 и DNS через прокси.\n* **Telegram:** будущая ручная настройка SOCKS5 в приложении.\n* **VS Code / Codex:** инструкции будут проверены отдельно; часть расширений и дочерних процессов может игнорировать прокси.\n* **Другое приложение:** необходима собственная поддержка SOCKS5 или HTTP.\n\nВыбор EXE не перехватывает трафик. UDP, QUIC и WebRTC могут обходить настройки приложения.")
	settings := widget.NewLabel("Автозапуск и автоподключение выключены.\n\nДемонстрация не изменяет маршруты, DNS, системный proxy или firewall.\nСлужбы и драйверы не используются.")
	settings.Wrapping = fyne.TextWrapWord
	diagnostics := widget.NewLabel("Core: независимый MockCore 0.1.0\nIPC: NDJSON v1, наследуемые stdin/stdout\nПрокси listeners: отсутствуют\nПроверка туннеля и handshake: не выполняются\nТелеметрия и загрузка стороннего core: отсутствуют")
	diagnostics.Wrapping = fyne.TextWrapWord
	about := widget.NewLabel("AWG Local Proxy · версия 0.1.0-demo\nGo + Fyne v2.8.2\n\nНезависимая оболочка. Код synacker/amnezia-proxy не включён.\nИнтеграция ядра заблокирована до разрешения лицензии.\n\nDPAPI CurrentUser защищает сохранённые секреты на диске, но не от программ с правами того же пользователя.")
	about.Wrapping = fyne.TextWrapWord
	tabs := container.NewAppTabs(container.NewTabItem("Подключение", container.NewPadded(connection)), container.NewTabItem("Профили", container.NewPadded(profilePage)), container.NewTabItem("Приложения", container.NewVScroll(apps)), container.NewTabItem("Настройки", container.NewPadded(settings)), container.NewTabItem("Диагностика", container.NewPadded(diagnostics)), container.NewTabItem("О программе", container.NewPadded(about)))
	w.SetContent(tabs)
	quitting := false
	quit := func() {
		if quitting {
			return
		}
		quitting = true
		w.SetCloseIntercept(nil)
		go func() { _ = core.Close(); fyne.Do(func() { a.Quit() }) }()
	}
	if desk, ok := a.(desktop.App); ok {
		desk.SetSystemTrayIcon(theme.ComputerIcon())
		desk.SetSystemTrayMenu(fyne.NewMenu("AWG Local Proxy", fyne.NewMenuItem("Открыть", func() { w.Show(); w.RequestFocus() }), fyne.NewMenuItem("Проверить демонстрацию", func() { run(true) }), fyne.NewMenuItem("Остановить демонстрацию", func() { run(false) }), fyne.NewMenuItem("Завершить", quit)))
		notified := false
		w.SetCloseIntercept(func() {
			w.Hide()
			if !notified {
				notified = true
				a.SendNotification(fyne.NewNotification("AWG Local Proxy", "Окно скрыто в трей. Для выхода выберите «Завершить»."))
			}
		})
	} else {
		w.SetCloseIntercept(quit)
	}
	return w
}
