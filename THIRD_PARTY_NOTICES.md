# Third-party notices

Проверка: 30 октября 2026 года. Цель: текущие локальные Windows x64 сборки GUI и отдельного ядра, Go 1.27.3, CGO/LLVM MinGW 20261020 UCRT. Этот файл и каталог `third_party/licenses/` следует сохранять рядом с распространяемыми EXE.

Лицензия собственного кода AWG Local Proxy и его независимо написанного core владельцем пока не выбрана. Перечисленные ниже лицензии относятся к сторонним компонентам и не лицензируют весь продукт. Код `synacker/amnezia-proxy` в эти компоненты не включён.

Инвентаризация основана на `go list -deps` для Windows-команд GUI и core: 26 внешних Go-модулей GUI и 8 core. Это множество пакетов, участвующих в компиляции; линкер может удалить неиспользуемые части. Оно не является перечнем DLL, доказательством загрузки драйвера или юридическим заключением. Исходные тексты найденных лицензий сохранены без изменения; `module-inventory.json` фиксирует версии и source origin, `manifest.json` — SHA-256 копий.

## Go-модули

Сохранены полные LICENSE/COPYING, а также найденные AUTHORS. Для dual-licensed go-text можно использовать BSD-3-Clause; оба исходных текста сохранены. Поля copyright и disclaimers находятся в связанных файлах.

| Сборка | Модуль и версия | Условия | Полные тексты |
|---|---|---|---|
| GUI | [fyne.io/fyne/v2 v2.8.2](https://github.com/fyne-io/fyne) | BSD-3-Clause | [AUTHORS](third_party/licenses/fyne.io_fyne_v2@v2.8.2/AUTHORS), [LICENSE](third_party/licenses/fyne.io_fyne_v2@v2.8.2/LICENSE) |
| GUI | [fyne.io/systray v1.12.3-0.20260810170012-af4e8e793ec4](https://github.com/fyne-io/systray) | Apache-2.0 | [LICENSE](third_party/licenses/fyne.io_systray@v1.12.3-0.20260810170012-af4e8e793ec4/LICENSE) |
| GUI | [github.com/anthonynsimon/bild v0.14.0](https://github.com/anthonynsimon/bild) | MIT | [LICENSE](third_party/licenses/github.com_anthonynsimon_bild@v0.14.0/LICENSE) |
| GUI | [github.com/BurntSushi/toml v1.6.0](https://github.com/BurntSushi/toml) | MIT | [COPYING](third_party/licenses/github.com_BurntSushi_toml@v1.6.0/COPYING) |
| GUI | [github.com/clipperhouse/uax29/v2 v2.2.0](https://github.com/clipperhouse/uax29) | MIT | [LICENSE](third_party/licenses/github.com_clipperhouse_uax29_v2@v2.2.0/LICENSE) |
| GUI | [github.com/fsnotify/fsnotify v1.9.0](https://github.com/fsnotify/fsnotify) | BSD-3-Clause | [LICENSE](third_party/licenses/github.com_fsnotify_fsnotify@v1.9.0/LICENSE) |
| GUI | [github.com/fyne-io/image v0.1.1](https://github.com/fyne-io/image) | BSD-3-Clause | [LICENSE](third_party/licenses/github.com_fyne-io_image@v0.1.1/LICENSE) |
| GUI | [github.com/fyne-io/oksvg v0.2.1-0.20260918172519-a9af55fa95d9](https://github.com/fyne-io/oksvg) | BSD-3-Clause | [LICENSE](third_party/licenses/github.com_fyne-io_oksvg@v0.2.1-0.20260918172519-a9af55fa95d9/LICENSE) |
| GUI | [github.com/FyshOS/fancyfs v0.0.1](https://github.com/FyshOS/fancyfs) | BSD-3-Clause | [AUTHORS](third_party/licenses/github.com_FyshOS_fancyfs@v0.0.1/AUTHORS), [LICENSE](third_party/licenses/github.com_FyshOS_fancyfs@v0.0.1/LICENSE) |
| GUI | [github.com/go-gl/gl v0.0.0-20260331235117-4566fea9a276](https://github.com/go-gl/gl) | MIT | [LICENSE](third_party/licenses/github.com_go-gl_gl@v0.0.0-20260331235117-4566fea9a276/LICENSE) |
| GUI | [github.com/go-gl/glfw/v3.4/glfw v0.1.0-pre.2](https://github.com/go-gl/glfw) | BSD-3-Clause (Go binding); additional C/header terms below | [LICENSE](third_party/licenses/github.com_go-gl_glfw_v3.4_glfw@v0.1.0-pre.2/LICENSE) |
| GUI | [github.com/go-text/render v0.2.1](https://github.com/go-text/render) | Unlicense OR BSD-3-Clause | [LICENSE](third_party/licenses/github.com_go-text_render@v0.2.1/LICENSE) |
| GUI | [github.com/go-text/typesetting v0.3.5](https://github.com/go-text/typesetting) | Unlicense OR BSD-3-Clause; HarfBuzz MIT | [LICENSE](third_party/licenses/github.com_go-text_typesetting@v0.3.5/LICENSE) |
| GUI | [github.com/godbus/dbus/v5 v5.2.2](https://github.com/godbus/dbus) | BSD-2-Clause | [LICENSE](third_party/licenses/github.com_godbus_dbus_v5@v5.2.2/LICENSE) |
| GUI | [github.com/jeandeaual/go-locale v0.0.0-20250612000132-0ef82f21eade](https://github.com/jeandeaual/go-locale) | MIT | [LICENSE](third_party/licenses/github.com_jeandeaual_go-locale@v0.0.0-20250612000132-0ef82f21eade/LICENSE) |
| GUI | [github.com/jsummers/gobmp v0.0.0-20230614200233-a9de23ed2e25](https://github.com/jsummers/gobmp) | MIT | [COPYING.txt](third_party/licenses/github.com_jsummers_gobmp@v0.0.0-20230614200233-a9de23ed2e25/COPYING.txt) |
| GUI | [github.com/mattn/go-runewidth v0.0.24](https://github.com/mattn/go-runewidth) | MIT | [LICENSE](third_party/licenses/github.com_mattn_go-runewidth@v0.0.24/LICENSE) |
| GUI | github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646 | ISC | [LICENSE](third_party/licenses/github.com_nfnt_resize@v0.0.0-20180221191011-83c6a9932646/LICENSE) |
| GUI | [github.com/nicksnyder/go-i18n/v2 v2.5.1](https://github.com/nicksnyder/go-i18n) | MIT | [LICENSE](third_party/licenses/github.com_nicksnyder_go-i18n_v2@v2.5.1/LICENSE) |
| GUI | [github.com/srwiley/oksvg v0.0.0-20221011165216-be6e8873101c](https://github.com/srwiley/oksvg) | BSD-3-Clause | [LICENSE](third_party/licenses/github.com_srwiley_oksvg@v0.0.0-20221011165216-be6e8873101c/LICENSE) |
| GUI | github.com/srwiley/rasterx v0.0.0-20220730225603-2ab79fcdd4ef | BSD-3-Clause | [LICENSE](third_party/licenses/github.com_srwiley_rasterx@v0.0.0-20220730225603-2ab79fcdd4ef/LICENSE) |
| GUI | [github.com/yuin/goldmark v1.8.2](https://github.com/yuin/goldmark) | MIT | [LICENSE](third_party/licenses/github.com_yuin_goldmark@v1.8.2/LICENSE) |
| GUI | [golang.org/x/image v0.43.0](https://go.googlesource.com/image) | BSD-3-Clause | [LICENSE](third_party/licenses/golang.org_x_image@v0.43.0/LICENSE) |
| GUI | [golang.org/x/net v0.35.0](https://go.googlesource.com/net) | BSD-3-Clause | [LICENSE](third_party/licenses/golang.org_x_net@v0.35.0/LICENSE) |
| GUI | [golang.org/x/sys v0.37.0](https://go.googlesource.com/sys) | BSD-3-Clause | [LICENSE](third_party/licenses/golang.org_x_sys@v0.37.0/LICENSE) |
| GUI | [golang.org/x/text v0.38.0](https://go.googlesource.com/text) | BSD-3-Clause | [LICENSE](third_party/licenses/golang.org_x_text@v0.38.0/LICENSE) |
| Core | [github.com/amnezia-vpn/amneziawg-go/v3 v3.1.20260828](https://github.com/amnezia-vpn/amneziawg-go) | MIT | [LICENSE](third_party/licenses/github.com_amnezia-vpn_amneziawg-go_v3@v3.1.20260828/LICENSE) |
| Core | [github.com/google/btree v1.1.3](https://github.com/google/btree) | Apache-2.0 | [LICENSE](third_party/licenses/github.com_google_btree@v1.1.3/LICENSE) |
| Core | [golang.org/x/crypto v0.42.0](https://go.googlesource.com/crypto) | BSD-3-Clause | [LICENSE](third_party/licenses/golang.org_x_crypto@v0.42.0/LICENSE) |
| Core | [golang.org/x/net v0.44.0](https://go.googlesource.com/net) | BSD-3-Clause | [LICENSE](third_party/licenses/golang.org_x_net@v0.44.0/LICENSE) |
| Core | [golang.org/x/sys v0.36.0](https://go.googlesource.com/sys) | BSD-3-Clause | [LICENSE](third_party/licenses/golang.org_x_sys@v0.36.0/LICENSE) |
| Core | [golang.org/x/time v0.9.0](https://go.googlesource.com/time) | BSD-3-Clause | [LICENSE](third_party/licenses/golang.org_x_time@v0.9.0/LICENSE) |
| Core | [golang.zx2c4.com/wintun v0.0.0-20230126152724-0fa3db229ce2](https://git.zx2c4.com/wintun-go) | MIT | [LICENSE](third_party/licenses/golang.zx2c4.com_wintun@v0.0.0-20230126152724-0fa3db229ce2/LICENSE) |
| Core | [gvisor.dev/gvisor v0.0.0-20231202080848-1f7806d17489](https://github.com/google/gvisor) | Apache-2.0 | [AUTHORS](third_party/licenses/gvisor.dev_gvisor@v0.0.0-20231202080848-1f7806d17489/AUTHORS), [LICENSE](third_party/licenses/gvisor.dev_gvisor@v0.0.0-20231202080848-1f7806d17489/LICENSE) |

Дополнительно сохранены исторические копии лицензий `golang.org/x/image v0.24.0` и `golang.org/x/text v0.22.0`. Эти версии не используются текущими Windows-сборками и не входят в текущий dependency graph; записи этих двух файлов в `manifest.json` помечены `historical_not_in_current_build`. Ссылки таблицы и `module-inventory.json` относятся к актуальным версиям `x/image v0.43.0` и `x/text v0.38.0`.

## Встроенные ресурсы и C-код

Дополнительные условия ниже дополняют верхние лицензии Go-модулей.

- **Noto Sans**: Copyright 2015 Google Inc. All Rights Reserved.; SIL Open Font License 1.1. [Copyright](third_party/licenses/fyne-fonts/NotoSans-COPYRIGHT.txt), [полный OFL](third_party/licenses/fyne-fonts/LICENSE.txt). Font name table Fyne содержит эту идентификацию. Шрифты нельзя продавать отдельно; при распространении сохраняются copyright и лицензия, для модифицированных шрифтов соблюдаются правила Reserved Font Names.
- **Inter Symbols**: SIL OFL 1.1, Copyright 2016–2020 The Inter Project Authors. [Полный текст](third_party/licenses/fyne-fonts/LICENSE_Inter.txt).
- **EmojiOneColor**: MIT, Copyright 2016 Adobe Systems Incorporated. [Полный текст](third_party/licenses/fyne-fonts/LICENSE_EmojiOneColor.txt).
- **DejaVu Sans Mono Powerline**: Bitstream Vera и Arev notices, DejaVu changes public domain. Сохраняются copyright/trademark notices и ограничения на названия модифицированных шрифтов. [Полные условия](third_party/licenses/fyne-fonts/LICENSE_DejaVu-Powerline.txt).
- **Native GLFW C**: zlib-style license. [Полный текст](third_party/licenses/native-glfw/LICENSE.md). Изменённые версии исходников должны быть обозначены; исходное авторство не приписывается продукту.
- **HarfBuzz в go-text**: MIT с собственным списком copyright holders. [Полный текст](third_party/licenses/harfbuzz/LICENSE).
- **Khronos platform header**: разрешительные условия с сохранением copyright и permission notice. [Полное уведомление](third_party/licenses/Khronos-khrplatform-NOTICE.txt).

### LGPL-заголовки GLFW на Windows

В GLFW binding `v0.1.0-pre.2`, commit `d41da22a9587f777098f96d37014f6cdd35d1afb`, Windows CFLAGS включают `glfw/deps/mingw`; native GLFW включает Wine-derived `dinput.h` и `xinput.h`. Они имеют LGPL-2.1-or-later: [dinput notice](third_party/licenses/native-glfw/dinput-header-NOTICE.txt), [xinput notice](third_party/licenses/native-glfw/xinput-header-NOTICE.txt), [полный LGPL 2.1](third_party/licenses/LGPL-2.1.txt).

В проверенных заголовках нет static/inline функций, максимальная длина макроса — две строки. LGPL 2.1 §5 допускает неограниченное использование object file, если использованы только числовые параметры, layouts/accessors и короткие macros/inline functions до десяти строк. Это основание для допустимости текущего header-only использования; это оценка фактов сборки, а не юридическая гарантия и не автоматическое лицензирование всего GUI под LGPL. При изменении headers, компилятора или способа связывания вывод нужно проверить заново; копирование/изменение самих LGPL-исходников имеет собственные обязательства.

Первичные источники: [Windows build flags](https://github.com/go-gl/glfw/blob/d41da22a9587f777098f96d37014f6cdd35d1afb/v3.4/glfw/build.go), [Wine-derived header](https://github.com/go-gl/glfw/blob/d41da22a9587f777098f96d37014f6cdd35d1afb/v3.4/glfw/glfw/deps/mingw/dinput.h), [GNU LGPL 2.1](https://www.gnu.org/licenses/old-licenses/lgpl-2.1.txt).

## Toolchain/runtime

- **Go 1.27.3 standard library/runtime**: BSD-3-Clause. [Go LICENSE](third_party/licenses/Go-LICENSE).
- **MinGW-w64 runtime**: собственный набор разрешительных notices. Его файл прямо требует включать перечисленные copyright/licensing notices при распространении статически связанного runtime. [Полный файл](third_party/licenses/COPYING.MinGW-w64-runtime.txt).
- **LLVM/compiler runtime**: Apache-2.0 с LLVM Exceptions и включёнными в toolchain notices. [Полный файл](third_party/licenses/LLVM-LICENSE.TXT). Файл сохранён для консервативного покрытия compiler-runtime; включение не утверждает, что все программы и компоненты LLVM toolchain входят в EXE. Сам toolchain не входит в продукт.
- Системные DLL Windows/UCRT этим комплектом не распространяются; их наличие в PE import table не делает их частью данного набора сторонних исходников. При добавлении DLL в архив требуется отдельная проверка прав на их распространение.

## Wintun Go bindings

`golang.zx2c4.com/wintun` присутствует в Windows dependency graph официального AmneziaWG через пакет `tun`. Проверенный Go wrapper имеет MIT LICENSE и SPDX MIT/copyright WireGuard LLC в исходных headers. [Лицензия wrapper](third_party/licenses/golang.zx2c4.com_wintun@v0.0.0-20230126152724-0fa3db229ce2/LICENSE), [copyright из headers](third_party/licenses/golang.zx2c4.com_wintun@v0.0.0-20230126152724-0fa3db229ce2/COPYRIGHT.txt). Его DLL loader ленивый; сама зависимость не доказывает создание адаптера или загрузку DLL. Продукт использует userspace `tun/netstack`, а не `tun.CreateTUN`. Никакие Wintun DLL или драйверы этим набором не включаются и не лицензируются; если они появятся в пакете, аудит потребуется отдельно.

## Область проверки и комплектование

В root Go-модулей проверенного compilation graph обнаружены явные лицензии. Известные вложенные лицензии ресурсов/C-кода и runtime включены выше. Не найдено основания считать source-license блокировку исходного `synacker/amnezia-proxy` снятой; она остаётся отдельным вопросом и не относится к выбранному независимому компоненту.

Этот набор не подтверждает результаты vulnerability/security audit, отсутствие любых невыявленных прав третьих лиц, совместимость с конкретным сервером, подпись бинарников или готовность публичного релиза. Любое изменение зависимостей, встроенных ресурсов или toolchain требует актуализации инвентаря и уведомлений.

Для локального пакета скопируйте `THIRD_PARTY_NOTICES.md` и весь `third_party/licenses/` с сохранением относительных путей рядом с GUI/core. Если архив содержит исходники сторонних компонентов, соблюдайте также их source-distribution условия. Проверяйте наличие этого набора в итоговом архиве; одни ссылки на сайты не заменяют приложенные тексты лицензий.
