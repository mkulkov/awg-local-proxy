# Проверка лицензии amnezia-proxy

Дата повторной проверки: 2026-10-30. Статус: `NO_EXPLICIT_LICENSE_FOUND`.

На проверенном HEAD `main` — `f5a7912dba8b128ae5c1a3f68a06be7c1668b908` — GitHub API сообщает `license: null`. Полное дерево API возвращено с `truncated: false`; LICENSE, COPYING, NOTICE и другие лицензионные файлы отсутствуют. README, девять Go-файлов и go.mod проверены на указания лицензии, copyright, SPDX и типовой текст разрешения; разрешающей лицензии не найдено. Отсутствие разрешения вне репозитория не доказано.

GitHub разрешает просмотр и fork публичного контента внутри сервиса. Допускается неизменённый GitHub fork. До подходящей лицензии на конкретную версию или документированного разрешения правообладателя изменение и распространение производного proxy, включение его исходников или бинарников в продукт и автоматическое скачивание компонента заблокированы. Самостоятельные GUI, контракты, хранилище и MockCore можно разрабатывать без копирования кода upstream. Разделение на процессы не снимает лицензионных ограничений.

Зависимость `amnezia-vpn/amneziawg-go` на указанном upstream теге имеет MIT LICENSE; это не лицензия на код synacker/amnezia-proxy. Полный аудит прямых и транзитивных зависимостей требуется перед выпуском и не завершён. Разрешение автора не запрошено; подготовленный черновик обращения не отправлен.

Источники:

- [Метаданные upstream](https://api.github.com/repos/synacker/amnezia-proxy)
- [HEAD main](https://api.github.com/repos/synacker/amnezia-proxy/commits/main)
- [Полное дерево проверенного commit](https://api.github.com/repos/synacker/amnezia-proxy/git/trees/f5a7912dba8b128ae5c1a3f68a06be7c1668b908?recursive=1)
- [README проверенного commit](https://github.com/synacker/amnezia-proxy/blob/f5a7912dba8b128ae5c1a3f68a06be7c1668b908/README.md)
- [Лицензия зависимости](https://github.com/amnezia-vpn/amneziawg-go/blob/v3.1.20260828/LICENSE)
- [GitHub Terms D.5](https://docs.github.com/en/site-policy/github-terms/github-terms-of-service#5-license-grant-to-other-users)
- [GitHub: лицензирование репозитория](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/licensing-a-repository)
