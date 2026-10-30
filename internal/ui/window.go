package ui

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/mkulkov/awg-local-proxy/internal/autostart"
	"github.com/mkulkov/awg-local-proxy/internal/coreclient"
	"github.com/mkulkov/awg-local-proxy/internal/preferences"
	"github.com/mkulkov/awg-local-proxy/internal/profiles"
	"github.com/mkulkov/awg-local-proxy/internal/session"
)

// localAddress accepts only a decimal TCP port and always constructs IPv4
// loopback; no user-supplied bind host can reach the core.
func localAddress(port string) (string, error) {
	if len(port) == 0 || len(port) > 5 {
		return "", errors.New("укажите порт от 1 до 65535")
	}
	for _, c := range port {
		if c < '0' || c > '9' {
			return "", errors.New("укажите порт от 1 до 65535")
		}
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", errors.New("укажите порт от 1 до 65535")
	}
	return "127.0.0.1:" + strconv.Itoa(n), nil
}

func localAddresses(socksPort, httpPort string, httpEnabled bool) (string, string, error) {
	socks, err := localAddress(socksPort)
	if err != nil {
		return "", "", errors.New("порт SOCKS5 должен быть от 1 до 65535")
	}
	if !httpEnabled {
		return socks, "", nil
	}
	http, err := localAddress(httpPort)
	if err != nil {
		return "", "", errors.New("порт HTTP должен быть от 1 до 65535")
	}
	if socks == http {
		return "", "", errors.New("порты SOCKS5 и HTTP должны различаться")
	}
	return socks, http, nil
}

func savedProfileIndex(items []profiles.Metadata, id string) int {
	if id == "" {
		return -1
	}
	for i, m := range items {
		if m.ID == id && !m.Demo {
			return i
		}
	}
	return -1
}

func New(a fyne.App, core coreclient.Client, real *session.Controller, store *profiles.Store, storageError error) fyne.Window {
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
	httpEnabled := widget.NewCheck("Включить HTTP proxy", nil)
	reconnect := widget.NewCheck("Повторять подключение при временной ошибке (до 5 попыток)", nil)
	socksPort := widget.NewEntry()
	socksPort.SetText("1080")
	httpPort := widget.NewEntry()
	httpPort.SetText("8080")
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
		for i, m := range items {
			names = append(names, strconv.Itoa(i+1)+" · "+m.Name)
		}
		profile.SetOptions(names)
		profile.SetSelectedIndex(0)
	}
	refreshProfiles()
	var prefStore *preferences.Store
	var saved preferences.Settings
	var prefErr error
	if real != nil {
		prefStore, prefErr = preferences.OpenDefault()
		if prefErr == nil {
			saved, prefErr = prefStore.LoadForAutoConnect(func(id string) error {
				if store == nil || savedProfileIndex(loaded, id) < 0 {
					return preferences.ErrProfileUnavailable
				}
				secret, err := store.ReadSecret(id)
				clear(secret)
				return err
			})
		}
		if prefErr != nil {
			saved = preferences.Settings{}
		}
		reconnect.SetChecked(saved.AutoReconnect)
		if saved.AutoConnect {
			i := savedProfileIndex(loaded, saved.ProfileID)
			if i >= 0 {
				profile.SetSelectedIndex(i + 1)
				probe.SetText(saved.ProbeURL)
				socksPort.SetText(strconv.Itoa(saved.SocksPort))
				httpPort.SetText(strconv.Itoa(saved.HTTPPort))
				httpEnabled.SetChecked(saved.HTTPEnabled)
			} else {
				saved = preferences.Settings{}
			}
		}
	}
	active := false
	busy := false
	quitting := false
	uiDone := make(chan struct{})
	var checkTunnel *widget.Button
	var socksCopy, httpCopy *widget.Button
	setPortControls := func(enabled bool) {
		if enabled {
			socksPort.Enable()
			httpPort.Enable()
			reconnect.Enable()
		} else {
			socksPort.Disable()
			httpPort.Disable()
			reconnect.Disable()
		}
	}
	disableAddressCopies := func() {
		if socksCopy != nil {
			socksCopy.Disable()
		}
		if httpCopy != nil {
			httpCopy.Disable()
		}
	}
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
		retryEnabled := reconnect.Checked
		probeURL := ""
		socksAddress := ""
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
			var err error
			socksAddress, httpAddress, err = localAddresses(socksPort.Text, httpPort.Text, httpEnabled.Checked)
			if err != nil {
				dialog.ShowInformation("Порты proxy", err.Error(), w)
				return
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
		setPortControls(false)
		disableAddressCopies()
		status.SetText("Подключение…")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			var s coreclient.Status
			var err error
			if connect && real != nil {
				s, err = real.Start(session.Selection{ProfileID: profileID, SocksAddress: socksAddress, HTTPAddress: httpAddress, ProbeURL: probeURL, Reconnect: retryEnabled})
			} else if connect {
				s, err = core.Connect(ctx)
			} else {
				s, err = core.Disconnect(ctx)
			}
			fyne.Do(func() {
				if quitting {
					return
				}
				busy = false
				primary.Enable()
				if !connect || err != nil {
					profile.Enable()
					probe.Enable()
					httpEnabled.Enable()
					setPortControls(true)
				}
				if err != nil {
					active = false
					primary.SetText(startLabel)
					status.SetText("Подключение не установлено")
					detail.SetText("Проверьте профиль и доступность сервера. Прямой обходной выход не используется.")
					if real != nil && s.Message != "" {
						detail.SetText(s.Message)
					}
					return
				}
				active = connect
				if connect {
					primary.SetText(stopLabel)
					if real == nil {
						status.SetText("Демонстрация активна · туннеля нет")
					} else if s.State == coreclient.Connected {
						status.SetText("Последняя проверка туннеля прошла")
					} else if s.State == coreclient.Connecting || s.State == coreclient.Starting {
						status.SetText("Подключение…")
					} else if s.State == coreclient.Reconnecting {
						status.SetText("Повтор подключения…")
					} else {
						status.SetText("Туннель не подтверждён")
					}
					if checkTunnel != nil && s.State == coreclient.Connected {
						checkTunnel.Enable()
					}
					if real != nil && s.ListenerReady {
						socksCopy.Enable()
						if httpAddress != "" {
							httpCopy.Enable()
						}
					}
				} else {
					primary.SetText(startLabel)
					status.SetText("Остановлено")
				}
				if real == nil {
					detail.SetText(s.Message)
				} else if connect {
					detail.SetText(s.Message)
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
		liveAddress := func(label string, port *widget.Entry) (fyne.CanvasObject, *widget.Button) {
			display := widget.NewLabel("")
			update := func(value string) {
				a, err := localAddress(value)
				if err != nil {
					display.SetText("Неверный порт")
				} else {
					display.SetText(a)
				}
			}
			update(port.Text)
			port.OnChanged = update
			copy := widget.NewButtonWithIcon("Копировать", theme.ContentCopyIcon(), func() {
				if a, err := localAddress(port.Text); err == nil {
					w.Clipboard().SetContent(a)
				}
			})
			copy.Disable()
			return container.NewBorder(nil, nil, widget.NewLabel(label), copy, display), copy
		}
		var socksRow, httpRow fyne.CanvasObject
		socksRow, socksCopy = liveAddress("SOCKS5", socksPort)
		httpRow, httpCopy = liveAddress("HTTP (по выбору)", httpPort)
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
					if quitting {
						return
					}
					busy = false
					primary.Enable()
					checkTunnel.Enable()
					if err != nil || s.State != coreclient.Connected {
						status.SetText("Проверка туннеля не прошла")
						if s.State == coreclient.Reconnecting {
							status.SetText("Повтор подключения…")
							checkTunnel.Disable()
							disableAddressCopies()
						}
						if err != nil || s.State == coreclient.Error || s.State == coreclient.Stopped {
							active = false
							primary.SetText(startLabel)
							checkTunnel.Disable()
							profile.Enable()
							probe.Enable()
							httpEnabled.Enable()
							setPortControls(true)
							disableAddressCopies()
						}
						return
					}
					status.SetText("Последняя проверка туннеля прошла")
				})
			}()
		})
		checkTunnel.Disable()
		retryHint := widget.NewLabel("Повтор использует тот же выбранный адрес проверки. Ошибки профиля, портов и отдельного адреса назначения не запускают повтор; отключение отменяет ожидание.")
		retryHint.Wrapping = fyne.TextWrapWord
		connection = container.NewVBox(widget.NewLabel("Профиль"), profile, widget.NewLabel("Адрес проверки через туннель"), probe, httpEnabled, reconnect, retryHint, status, primary, checkTunnel, detail, widget.NewSeparator(), socksRow, httpRow, widget.NewLabel("Порты меняются в настройках. Прокси работает только при активном подключении. UDP не поддерживается."))
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
	profileInfo := widget.NewLabel("Можно сохранить demo-профиль или ограниченный клиентский .conf в DPAPI. Импорт не запускает VPN. Один Peer, поддерживаемые параметры J/S/H/I. Неизвестные поля и .vpn не поддержаны.")
	if real != nil {
		profileInfo.SetText("Импортируйте клиентский .conf размером до 8 КиБ. Секрет будет зашифрован для текущего пользователя; исходный файл останется на месте. Поддерживается ограниченный набор полей и один Peer. Имя VPN-сервера при подключении разрешается системным DNS; используйте IP-адрес, чтобы исключить этот запрос.")
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
		apps.ParseMarkdown("### Настройка приложений\nСкопируйте адрес SOCKS5 со вкладки подключения и включите разрешение имён через прокси, если оно поддерживается. HTTP proxy включается отдельно.\n\n* **Браузер:** используйте отдельный профиль и настройки прокси.\n* **Telegram:** укажите SOCKS5 в настройках соединения.\n* **VS Code / Codex:** отдельные расширения и процессы могут игнорировать прокси.\n\nВыбор EXE не перехватывает трафик. UDP, QUIC, WebRTC и DNS самого приложения могут идти в обход.")
		settings.SetText("HTTP proxy включается на вкладке подключения.\n\nАвтозапуск интерфейса, автоподключение и повтор соединения — раздельные настройки. Системные маршруты, DNS, proxy и firewall не изменяются.")
		diagnostics.SetText("Core: отдельный процесс AmneziaWG\nIPC: NDJSON v1, наследуемые stdin/stdout\nПодключение подтверждается handshake и запросом по заданному вами адресу.\nТелеметрия и загрузка core при запуске отсутствуют.")
		about.SetText("AWG Local Proxy · предварительная версия\nGo + Fyne v2.8.2\n\nОфициальный движок AmneziaWG; код synacker/amnezia-proxy не включён.\n\nDPAPI не защищает секреты от программ с правами того же пользователя.")
	}
	settingsPage := fyne.CanvasObject(settings)
	if real != nil {
		startupSync := true
		var startup *widget.Check
		startup = widget.NewCheck("Запускать интерфейс при входе в Windows", func(enabled bool) {
			if startupSync {
				return
			}
			startup.Disable()
			go func() {
				var err error
				if enabled {
					err = autostart.Enable()
				} else {
					err = autostart.Disable()
				}
				actual, readErr := autostart.Get()
				fyne.Do(func() {
					if quitting {
						return
					}
					startupSync = true
					startup.SetChecked(actual)
					startupSync = false
					startup.Enable()
					if err != nil || readErr != nil {
						message := "Не удалось изменить автозапуск. Состояние восстановлено; подключение не запускается."
						if errors.Is(err, autostart.ErrConflict) || errors.Is(readErr, autostart.ErrConflict) {
							message = "Запись автозапуска принадлежит другой копии приложения и не была изменена."
						}
						dialog.ShowInformation("Автозапуск", message, w)
					}
				})
			}()
		})
		startupEnabled, startupErr := autostart.Get()
		startup.SetChecked(startupEnabled)
		startupSync = false
		startupInfo := widget.NewLabel("Автозапуск открывает только интерфейс и не подключает профиль.")
		startupInfo.Wrapping = fyne.TextWrapWord
		if startupErr != nil {
			startupInfo.SetText("Состояние автозапуска недоступно или запись принадлежит другой копии. Изменения не выполнялись.")
		}
		autoConnect := widget.NewCheck("Подключать сохранённый профиль при запуске", nil)
		autoConnect.SetChecked(saved.AutoConnect)
		prefInfo := widget.NewLabel("Сохранение автоподключения разрешает при следующем запуске запрос к выбранному сейчас адресу проверки. Настройки защищены DPAPI. Кнопка «Отключить» останавливает текущий сеанс; для отмены следующего автоподключения снимите флажок и сохраните настройки.")
		prefInfo.Wrapping = fyne.TextWrapWord
		if prefErr != nil {
			prefInfo.SetText("Защищённые настройки не прочитаны. Автоподключение отключено; выберите профиль и сохраните настройки заново.")
		}
		var saveStartup *widget.Button
		saveStartup = widget.NewButton("Сохранить настройки подключения", func() {
			if prefStore == nil {
				dialog.ShowInformation("Настройки", "Защищённое хранилище недоступно. Автоподключение не сохранено.", w)
				return
			}
			if active || busy {
				dialog.ShowInformation("Сначала отключитесь", "Сохраните настройки после отключения текущего соединения.", w)
				return
			}
			value := preferences.Settings{AutoConnect: autoConnect.Checked, AutoReconnect: reconnect.Checked}
			if value.AutoConnect {
				i := profile.SelectedIndex() - 1
				if i < 0 || i >= len(loaded) || loaded[i].Demo {
					dialog.ShowInformation("Настройки", "Выберите импортированный профиль на вкладке подключения.", w)
					return
				}
				sp, se := strconv.Atoi(socksPort.Text)
				hp, he := strconv.Atoi(httpPort.Text)
				value.ProfileID = loaded[i].ID
				value.ProbeURL = probe.Text
				value.SocksPort = sp
				value.HTTPPort = hp
				value.HTTPEnabled = httpEnabled.Checked
				if se != nil || he != nil || value.Validate() != nil {
					dialog.ShowInformation("Настройки", "Проверьте адрес HTTP(S), оба порта и выбранный профиль.", w)
					return
				}
			}
			saveStartup.Disable()
			go func() {
				var err error
				if value.AutoConnect {
					var secret []byte
					secret, err = store.ReadSecret(value.ProfileID)
					clear(secret)
				}
				if err == nil {
					err = prefStore.Save(value)
				}
				fyne.Do(func() {
					if quitting {
						return
					}
					saveStartup.Enable()
					if err != nil {
						dialog.ShowInformation("Настройки", "Не удалось сохранить защищённые настройки. Автоподключение не изменено.", w)
						autoConnect.SetChecked(saved.AutoConnect)
						return
					}
					saved = value
					prefInfo.SetText("Настройки сохранены. Автозапуск интерфейса отдельно управляется флажком выше.")
				})
			}()
		})
		if prefStore == nil {
			saveStartup.Disable()
			autoConnect.Disable()
		}
		portHint := widget.NewLabel("Порты можно изменить только после отключения. После перезапуска они сбрасываются на 1080 и 8080.")
		portHint.SetText("Порты можно изменить только после отключения. Для автоподключения сохраняются выбранные значения; иначе после запуска используются 1080 и 8080.")
		portHint.Wrapping = fyne.TextWrapWord
		settingsPage = container.NewVBox(settings, startup, startupInfo, autoConnect, prefInfo, saveStartup, widget.NewLabel("Порт SOCKS5 (127.0.0.1)"), socksPort, widget.NewLabel("Порт HTTP (127.0.0.1)"), httpPort, portHint)
	}
	tabs := container.NewAppTabs(container.NewTabItem("Подключение", container.NewVScroll(container.NewPadded(connection))), container.NewTabItem("Профили", container.NewVScroll(container.NewPadded(profilePage))), container.NewTabItem("Приложения", container.NewVScroll(apps)), container.NewTabItem("Настройки", container.NewVScroll(container.NewPadded(settingsPage))), container.NewTabItem("Диагностика", container.NewPadded(diagnostics)), container.NewTabItem("О программе", container.NewPadded(about)))
	w.SetContent(tabs)
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
									setPortControls(true)
									disableAddressCopies()
									return
								}
								if s.State == coreclient.Degraded {
									status.SetText("Туннель требует проверки")
								}
								if s.State == coreclient.Connecting || s.State == coreclient.Starting || s.State == coreclient.Reconnecting {
									if s.State == coreclient.Reconnecting {
										status.SetText("Повтор подключения…")
									} else {
										status.SetText("Подключение…")
									}
									detail.SetText(s.Message)
									checkTunnel.Disable()
									disableAddressCopies()
								}
								if s.State == coreclient.Connected {
									status.SetText("Последняя проверка туннеля прошла")
									detail.SetText("Прокси доступен только локально. Другие приложения не перенаправляются автоматически.")
									checkTunnel.Enable()
									socksCopy.Enable()
									if s.HTTPAddress != "" {
										httpCopy.Enable()
									}
								}
								if s.State == coreclient.Stopped || s.State == coreclient.Error {
									active = false
									status.SetText("Подключение остановлено")
									primary.SetText(startLabel)
									checkTunnel.Disable()
									profile.Enable()
									probe.Enable()
									httpEnabled.Enable()
									setPortControls(true)
									disableAddressCopies()
								}
							})
						}()
					})
				}
			}
		}()
	}
	if real != nil && saved.AutoConnect {
		// Loading is read-only. Only an exact, previously saved and decryptable
		// non-demo selection may initiate this one startup action.
		startupSettings := saved
		fyne.Do(func() {
			if quitting || active || busy {
				return
			}
			i := profile.SelectedIndex() - 1
			if i < 0 || i >= len(loaded) || loaded[i].ID != startupSettings.ProfileID || loaded[i].Demo {
				return
			}
			if probe.Text != startupSettings.ProbeURL || socksPort.Text != strconv.Itoa(startupSettings.SocksPort) || httpPort.Text != strconv.Itoa(startupSettings.HTTPPort) || httpEnabled.Checked != startupSettings.HTTPEnabled || reconnect.Checked != startupSettings.AutoReconnect {
				return
			}
			run(true)
		})
	}
	return w
}
