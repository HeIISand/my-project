## Сборка и запуск

### Локальный запуск

```bash
cd service
go build -o ghreport .
./ghreport --repos cli/cli --limit 20 --out -
```

Проверка тестов и статического анализа:

```bash
cd service
go test ./...
gofmt -l .
go vet ./...
```

### Запуск в Docker

```bash
docker build -t ghreport service/
docker run --rm -e GITHUB_TOKEN=... ghreport --repos cli/cli --limit 20 --out -
```

При работе с альтернативным сервером API:

```bash
docker run --rm -e GITHUB_TOKEN=... -e GITHUB_API_BASE_URL=http://localhost:8080 ghreport --repos cli/cli --out -
```

## Параметры командной строки

- `--repos` — обязательный список репозиториев через запятую в формате `owner/repo` (дубликаты удаляются с сохранением первого вхождения).
- `--limit` — максимальное число issue на репозиторий (по умолчанию `100`, целое число `>= 1`).
- `--since` — фильтр по дате создания `YYYY-MM-DD` (включаются issue, созданные не раньше `00:00:00 UTC` указанной даты).
- `--out` — путь к файлу отчёта или `-` для вывода в `stdout` (по умолчанию `-`).
- `--trace` — путь к файлу журнала HTTP-запросов в формате JSON Lines (по умолчанию не ведётся).
- `--cache-dir` — каталог для хранения HTTP-кэша (по умолчанию `./.ghreport-cache`).

## Переменные окружения

- `GITHUB_TOKEN` — необязательный токен доступа GitHub REST API. При наличии передаётся в заголовке `Authorization: Bearer <token>`. Никогда не попадает в `stdout`, `stderr`, журнал `--trace` и файлы кэша.
- `GITHUB_API_BASE_URL` — базовый адрес API (по умолчанию `https://api.github.com`).

## Архитектура

Служба разделена на логические модули внутри пакета `main`:

- `models.go` — структуры данных для конфигурации, отчёта (`Report`, `RepoReport`, `Issue`), журнала (`TraceEntry`), кэша (`CacheEntry`) и ответов GitHub API.
- `config.go` — разбор и валидация флагов командной строки (`parseArgs`, `isValidRepo`) и чтение переменных окружения.
- `cache.go` — чтение и запись файлов HTTP-кэша (`loadCache`, `saveCache`) и логирование попыток в `--trace` (`writeTrace`).
- `client.go` — HTTP-клиент, разбор заголовка `Link` (`parseNextLink`), обработка лимитов и повторных попыток (`fetchPage`), фильтрация pull request-ов и дат `--since`, дедупликация и сортировка (`processRepo`).
- `main.go` — точка входа (`main`, `run`), управление кодами возврата (`0`, `1`, `2`, `3`) и вывод итогового отчёта.

## Формат кэша

Для каждого запрашиваемого URL вычисляется SHA-256 хеш, который служит именем файла `<sha256>.json` внутри `--cache-dir`. В файле хранится JSON-объект со структурой:

```json
{
  "etag": "\"...\"",
  "link": "<...>; rel=\"next\"",
  "body": "..."
}
```

## Обработка лимитов и повторов

- **Ограничение частоты**: служба определяет паузу по заголовку `Retry-After`, при его отсутствии — по разнице до `X-RateLimit-Reset`, иначе ждёт 60 секунд. Выполняется до 3 повторных попыток и всего до 4 попыток на запрос. При исчерпании попыток репозиторию выставляется ошибка `"rate_limited"`.
- **Сетевые ошибки и ответы `5xx`**: выполняется до 3 повторных попыток с экспоненциальной задержкой `1`, `2` и `4` секунды. При неудаче выставляется `"request_failed"`.

## Примеры вывода

Успешный отчёт (код возврата `0`, либо `1` при частичных ошибках):

```json
{
  "generated_at": "2026-10-14T09:00:00Z",
  "repos": [
    {
      "repo": "cli/cli",
      "error": null,
      "count": 1,
      "issues": [
        {
          "number": 1,
          "title": "Первый выпуск",
          "author": "octocat",
          "labels": ["bug"],
          "created_at": "2026-01-05T10:00:00Z",
          "url": "https://github.com/cli/cli/issues/1"
        }
      ]
    }
  ]
}
```

Фатальная ошибка (код возврата `3`):

```json
{"error": "unauthorized"}
```

```json
{"error": "all_failed"}
```
