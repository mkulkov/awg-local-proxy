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

func New(a fyne.App, core coreclient.Client, real *coreclient.RealSupervisor, store *profiles.Store, storageError error) fyne.Window {
	title := appTitle
	if real != nil {
		title = "AWG Local Proxy"
	}
	w := a.NewWindow(title)
	w.Resize(fyne.NewSize(700, 540))
	w.SetMaster()
	status := widget.NewLabel("Остановлено")
	detail := widget.NewLabel(demoWarning)
	detail.Wrapping = fyne.TextWrapWord
	defaultProfile := "Демонстрационный профиль"
	if real != nil {
		defaultProfile = "Выберите профиль"
	}
	profile := widget.NewSelect([]string{defaultProfile}, nil)
	profile.SetSelectedIndex(0)
	probe := widget.NewEntry()
	probe.SetPlaceHolder("https://адрес-проверки/")
	httpEnabled := widget.NewCheck("Включить HTTP proxy на 127.0.0.1:8080", nil)
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
		names := []string{defaultProfile}
		for _, m := range items {
			names = append(names, m.Name)
		}
		profile.SetOptions(names)
		profile.SetSelectedIndex(0)
	}
	refreshProfiles()
	active := false
	busy := false
	uiDone := make(chan struct{})
	var checkTunnel *widget.Button
	startLabel, stopLabel := connectLabel, disconnectLabel
	if real != nil {
		startLabel, stopLabel = "Подключить", "Отключить"
	}
	var primary *widget.Button
	run := func(connect bool) {
		if busy {
			return
		}
		profileID := ""
		probeURL := ""
		httpAddress := ""
		if real != nil && connect {
			i := profile.SelectedIndex() - 1
			if store == nil || i < 0 || i >= len(loaded) || loaded[i].Demo {
				dialog.ShowInformation("Профиль", "Выберите импортированный клиентский профиль.", w)
				return
			}
			if probe.Text == "" {
				dialog.ShowInformation("Проверка туннеля", "Укажите адрес HTTP(S), доступный через туннель. Он будет запрошен при подключении.", w)
				return
			}
			profileID, probeURL = loaded[i].ID, probe.Text
			if httpEnabled.Checked {
				httpAddress = "127.0.0.1:8080"
			}
		}
		primary.Disable()
		busy = true
		if checkTunnel != nil {
			checkTunnel.Disable()
		}
		profile.Disable()
		probe.Disable()
		httpEnabled.Disable()
		status.SetText("Подключение…")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			var s coreclient.Status
			var err error
			if connect && real != nil {
				var secret []byte
				secret, err = store.ReadSecret(profileID)
				if err == nil {
					s, err = real.ConnectProfile(ctx, coreclient.ConnectRequest{Config: secret, SocksAddress: "127.0.0.1:1080", HTTPAddress: httpAddress, ProbeURL: probeURL, TimeoutSeconds: 15})
				}
				clear(secret)
			} else if connect {
				s, err = core.Connect(ctx)
			} else {
				s, err = core.Disconnect(ctx)
			}
			fyne.Do(func() {
				busy = false
				primary.Enable()
				if !connect || err != nil {
					profile.Enable()
					probe.Enable()
					httpEnabled.Enable()
				}
				if err != nil {
					active = false
					primary.SetText(startLabel)
					status.SetText("Подключение не установлено")
					detail.SetText("Проверьте профиль и доступность сервера. Прямой обходной выход не используется.")
					return
				}
				active = connect
				if connect {
					primary.SetText(stopLabel)
					if real == nil {
						status.SetText("Демонстрация активна · туннеля нет")
					} else if s.State == coreclient.Connected {
						status.SetText("Последняя проверка туннеля прошла")
					} else {
						status.SetText("Туннель не подтверждён")
					}
					if checkTunnel != nil {
						checkTunnel.Enable()
					}
				} else {
					primary.SetText(startLabel)
					status.SetText("Остановлено")
				}
				if real == nil {
					detail.SetText(s.Message)
				} else if connect {
					detail.SetText("Прокси доступен только локально. Другие приложения не перенаправляются автоматически.")
				} else {
					detail.SetText("Прокси остановлен.")
				}
			})
		}()
	}
	primary = widget.NewButtonWithIcon(startLabel, theme.MediaPlayIcon(), func() { run(!active) })
	primary.Importance = widget.HighImportance
	address := func(label, value string) fyne.CanvasObject {
		entry := widget.NewLabel(value)
		copy := widget.NewButtonWithIcon("Копировать", theme.ContentCopyIcon(), func() { w.Clipboard().SetContent(value) })
		if real == nil {
			copy.Disable()
		}
		return container.NewBorder(nil, nil, widget.NewLabel(label), copy, entry)
	}
	connection := container.NewVBox(widget.NewLabelWithStyle(demoWarning, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), widget.NewSeparator(), widget.NewLabel("Профиль"), profile, status, primary, detail, widget.NewSeparator(), address("SOCKS5", "127.0.0.1:1080"), address("HTTP (выключен)", "127.0.0.1:8080"), widget.NewLabel("Адреса запланированы. В демонстрации порты не открыты."))
	if real != nil {
		primary.SetText("Подключить")
		detail.SetText("Выберите профиль и задайте адрес проверки туннеля.")
		checkTunnel = widget.NewButton("Проверить туннель ещё раз", func() {
			if !active || busy {
				return
			}
			busy = true
			primary.Disable()
			checkTunnel.Disable()
			status.SetText("Проверка туннеля…")
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				s, err := real.CheckTunnel(ctx)
				fyne.Do(func() {
					busy = false
					primary.Enable()
					checkTunnel.Enable()
					if err != nil || s.State != coreclient.Connected {
						status.SetText("Проверка туннеля не прошла")
						if err != nil || s.State == coreclient.Error || s.State == coreclient.Stopped {
							active = false
							primary.SetText(startLabel)
							checkTunnel.Disable()
							profile.Enable()
							probe.Enable()
							httpEnabled.Enable()
						}
						return
					}
					status.SetText("Последняя проверка туннеля прошла")
				})
			}()
		})
		checkTunnel.Disable()
		connection = container.NewVBox(widget.NewLabel("Профиль"), profile, widget.NewLabel("Адрес проверки через туннель"), probe, httpEnabled, status, primary, checkTunnel, detail, widget.NewSeparator(), address("SOCKS5", "127.0.0.1:1080"), address("HTTP (по выбору)", "127.0.0.1:8080"), widget.NewLabel("Прокси работает только при активном подключении. UDP не поддерживается."))
	}
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
	profileInfo := widget.NewLabel("Можно сохранить demo-профиль или ограниченный клиентский .conf в DPAPI. Импорт не запускает VPN. Один Peer, числовой Endpoint, поддерживаемые параметры J/S/H/I. Неизвестные поля и .vpn не поддержаны.")
	if real != nil {
		profileInfo.SetText("Импортируйте клиентский .conf размером до 8 КиБ. Секрет будет зашифрован для текущего пользователя; исходный файл останется на месте. Поддерживается ограниченный набор полей и один Peer.")
		create.Hide()
		name.Hide()
	}
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
				data, err := io.ReadAll(io.LimitReader(reader, profiles.MaxImportBytes+1))
				defer clear(data)
				if err == nil && len(data) > profiles.MaxImportBytes {
					err = errors.New("файл превышает 8 КиБ")
					userError = err.Error()
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
		if active || busy {
			dialog.ShowInformation("Сначала отключитесь", "Остановите подключение перед удалением профиля.", w)
			return
		}
		id := loaded[i].ID
		dialog.ShowConfirm("Удалить профиль?", "Будут удалены метаданные и зашифрованная копия. Исходный импортированный файл останется.", func(ok bool) {
			if ok {
				if active || busy {
					dialog.ShowInformation("Сначала отключитесь", "Остановите подключение перед удалением профиля.", w)
					return
				}
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
	if real != nil {
		apps.ParseMarkdown("### Настройка приложений\nУкажите SOCKS5 `127.0.0.1:1080` в приложении и включите разрешение имён через прокси, если оно поддерживается. HTTP proxy включается отдельно.\n\n* **Браузер:** используйте отдельный профиль и настройки прокси.\n* **Telegram:** укажите SOCKS5 в настройках соединения.\n* **VS Code / Codex:** отдельные расширения и процессы могут игнорировать прокси.\n\nВыбор EXE не перехватывает трафик. UDP, QUIC, WebRTC и DNS самого приложения могут идти в обход.")
		settings.SetText("HTTP proxy включается на вкладке подключения.\n\nАвтозапуск и автоподключение отсутствуют. Системные маршруты, DNS, proxy и firewall не изменяются.")
		diagnostics.SetText("Core: отдельный процесс AmneziaWG\nIPC: NDJSON v1, наследуемые stdin/stdout\nПодключение подтверждается handshake и запросом по заданному вами адресу.\nТелеметрия и загрузка core при запуске отсутствуют.")
		about.SetText("AWG Local Proxy · предварительная версия\nGo + Fyne v2.8.2\n\nОфициальный движок AmneziaWG; код synacker/amnezia-proxy не включён.\n\nDPAPI не защищает секреты от программ с правами того же пользователя.")
	}
	tabs := container.NewAppTabs(container.NewTabItem("Подключение", container.NewVScroll(container.NewPadded(connection))), container.NewTabItem("Профили", container.NewVScroll(container.NewPadded(profilePage))), container.NewTabItem("Приложения", container.NewVScroll(apps)), container.NewTabItem("Настройки", container.NewPadded(settings)), container.NewTabItem("Диагностика", container.NewPadded(diagnostics)), container.NewTabItem("О программе", container.NewPadded(about)))
	w.SetContent(tabs)
	quitting := false
	quit := func() {
		if quitting {
			return
		}
		quitting = true
		close(uiDone)
		w.SetCloseIntercept(nil)
		go func() { _ = core.Close(); fyne.Do(func() { a.Quit() }) }()
	}
	if desk, ok := a.(desktop.App); ok {
		desk.SetSystemTrayIcon(theme.ComputerIcon())
		desk.SetSystemTrayMenu(fyne.NewMenu("AWG Local Proxy", fyne.NewMenuItem("Открыть", func() { w.Show(); w.RequestFocus() }), fyne.NewMenuItem(startLabel, func() {
			if !active {
				run(true)
			}
		}), fyne.NewMenuItem(stopLabel, func() {
			if active {
				run(false)
			}
		}), fyne.NewMenuItem("Завершить", quit)))
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
	if real != nil {
		// Poll only local IPC; the core never sends an automatic network probe.
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-uiDone:
					return
				case <-ticker.C:
					fyne.Do(func() {
						if quitting || !active || busy {
							return
						}
						busy = true
						go func() {
							ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
							s, err := real.Status(ctx)
							cancel()
							fyne.Do(func() {
								busy = false
								if quitting {
									return
								}
								if err != nil {
									active = false
									status.SetText("Процесс ядра недоступен")
									detail.SetText("Перезапустите приложение. Прямой обходной выход не используется.")
									primary.Disable()
									checkTunnel.Disable()
									profile.Enable()
									probe.Enable()
									httpEnabled.Enable()
									return
								}
								if s.State == coreclient.Degraded {
									status.SetText("Туннель требует проверки")
								}
								if s.State == coreclient.Stopped || s.State == coreclient.Error {
									active = false
									status.SetText("Подключение остановлено")
									primary.SetText(startLabel)
									checkTunnel.Disable()
									profile.Enable()
									probe.Enable()
									httpEnabled.Enable()
								}
							})
						}()
					})
				}
			}
		}()
	}
	return w
}
