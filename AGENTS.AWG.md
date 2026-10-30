# AWG Local Proxy — проектные правила Codex

Рабочий каталог продукта: `C:\dev\GO\AwgLocalProxy`. Прочитай `CODEX_INSTRUCTIONS.md` перед первой задачей; затем перечитывай только нужные разделы. Текущий лицензионный статус — в `docs/LICENSE_AUDIT.md`.

## Инварианты

- Вариант А: только явно настроенные SOCKS5/HTTP приложения. Никакого перехвата EXE.
- Не изменять системные маршруты, DNS, WinINET/WinHTTP proxy, PAC и firewall. Никаких TUN/TAP/Wintun/WFP/WinDivert, служб и UAC для обычной работы.
- GUI и прокси — отдельные процессы и отдельные репозитории. Upstream-код не копировать в продукт.
- Для `synacker/amnezia-proxy` без лицензии допускается только неизменённый GitHub fork; изменение и распространение его кода заблокировано до разрешения. Пользователь отдельно разрешил независимый core на официальном AmneziaWG: см. `docs/CORE_SELECTION.md`. Не назначать свою лицензию чужому коду.
- Сетевые запросы назначения и их DNS — только внутри userspace-туннеля. При отказе ошибка, не DIRECT fallback. Endpoint bootstrap DNS — отдельное документированное исключение.
- Не передавать ключи через CLI/env и не сохранять plaintext config. DPAPI CurrentUser, ограниченные IPC pipes, редактированные логи.
- Никогда не коммитить конфиги, ключи, логи и захваты трафика. Проверять staged diff перед push.
- Не менять глобальные настройки Codex, model-router и сторонние проекты. Не устанавливать глобально навыки этого проекта.
- Не удалять существующие файлы, репозитории, ветки или работающие приложения для упрощения задачи. Не выполнять force push/reset --hard.
- Не выдавать MockCore, открытый порт или device.Up за рабочее VPN-подключение.

## Навыки по контексту

Выбирай один основной навык, максимум ещё один вспомогательный. Не загружай все тела SKILL.md заранее.

| Задача | Навык |
|---|---|
| Go networking, parser, netstack, DNS, IPC | `$awg-go-netstack` |
| Fyne UI, tray, Windows lifecycle, launch profiles | `$awg-windows-fyne` |
| Threat model, DPAPI, leaks, security acceptance | `$awg-security-tests` |
| License audit, GitHub fork, remotes, upstream | `$awg-github-upstream` |
| CI, build, lock, artifacts, release | `$awg-release-ci` |

UI-навык применяется только к UI/desktop задачам. Навыки не отменяют инструкции пользователя, внешние approval requirements и действующие глобальные правила. Если задача меняет код соседнего fork, прочитай применимые инструкции самого fork и нужный навык из этого проекта; не добавляй автоматически свои AGENTS в чужую основную ветку.

## Проверка работы

До первого push: правильный owner/visibility/fork status, `.gitignore`, лицензии, secret scan. Для кода: форматирование, unit tests, targeted integration tests; для GUI — Windows manual verification; для релиза — полный acceptance и чистая сборка.

Команды и версии определяй по фактическому репозиторию. Не запускай команды из документа механически до появления нужных каталогов/инструментов. Не скачивай и не исполняй произвольные installer scripts.

В отчёте разделяй выполнено, не проверено и заблокировано. Готовность показывай по фактическим результатам, не по наличию файлов.
