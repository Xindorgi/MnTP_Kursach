# URL Shortener 
Высокопроизводительный сервис сокращения ссылок с аналитикой на Go.

**Вариант 29.** Сервис сокращения ссылок с аналитикой

Аналог bit.ly. Go-бэкенд с генерацией коротких кодов,
редиректами, сбором статистики (переходы, гео, рефереры). Redis для
кэширования популярных ссылок. Дашборд для владельцев ссылок.

**Технологии:** Go + PostgreSQL + Redis

## Возможности

- **Сокращение URL** — создание коротких ссылок через REST API
- **Редирект** — мгновенное перенаправление с короткого кода на оригинальный URL (301 Moved Permanently)
- **Аналитика** — сбор статистики переходов: количество кликов, география, рефереры, ежедневная разбивка
- **Веб-дашборд** — встроенная HTML-панель для тестирования API
- **Кэширование** — Redis для быстрого разрешения коротких ссылок (с fallback на in-memory)
- **GeoIP** — определение страны и города по IP через GeoLite2 (MaxMind)
- **Graceful Shutdown** — корректное завершение работы с дообработкой очереди событий
- **Batch-вставка** — асинхронная запись кликов в БД батчами (до 50 событий или раз в 1 секунду)
- **In-memory fallback** — при недоступности PostgreSQL/Redis сервис продолжает работу в памяти
- **Docker Compose** — полная инфраструктура одной командой

## Архитектура

```
┌─────────────┐     ┌──────────────┐     ┌────────────┐
│   Клиент    │────▶│  Fiber HTTP  │────▶│  Service   │
│ (браузер/   │     │   Server     │     │  (бизнес-  │
│  curl/      │◀────│  (handlers)  │◀────│   логика)  │
│  клиент)    │     └──────────────┘     └──────┬─────┘
└─────────────┘                                 │
                                                 │
                    ┌────────────────────────────┼────────────────────┐
                    │                            │                    │
                    ▼                            ▼                    ▼
           ┌──────────────┐            ┌──────────────┐     ┌──────────────┐
           │  PostgreSQL  │            │    Redis     │     │  Analytics   │
           │  (основное   │            │   (кэш)      │     │   Worker     │
           │  хранилище)  │            │              │     │  (GeoIP +    │
           └──────────────┘            └──────────────┘     │  Batch)      │
                                                            └──────┬───────┘
                                                                   │
                                                                   ▼
                                                           ┌──────────────┐
                                                           │  PostgreSQL  │
                                                           │  (click_     │
                                                           │   events)    │
                                                           └──────────────┘
```

### Компоненты

| Компонент | Технология | Назначение |
|-----------|-----------|------------|
| **HTTP-сервер** | Fiber v2 | Обработка запросов, маршрутизация, middleware |
| **Бизнес-логика** | Service layer | Создание/резолвинг URL, валидация, управление токенами |
| **Хранилище URL** | PostgreSQL + in-memory fallback | Персистентное хранение ссылок |
| **Хранилище кликов** | PostgreSQL + in-memory fallback | Персистентное хранение аналитики |
| **Кэш** | Redis + in-memory fallback | Быстрое разрешение коротких кодов |
| **Analytics Worker** | Go-горутина | Асинхронная обработка кликов: GeoIP → batch insert |
| **Генерация кодов** | Sqids (Hashids) | Короткие уникальные коды из числовых ID |

## Технологический стек

- **Язык:** Go 1.26
- **Веб-фреймворк:** [Fiber v2](https://github.com/gofiber/fiber)
- **База данных:** PostgreSQL 16 (через [pgx v5](https://github.com/jackc/pgx))
- **Кэш:** Redis 7 (через [go-redis v9](https://github.com/redis/go-redis))
- **GeoIP:** [geoip2-golang](https://github.com/oschwald/geoip2-golang) + GeoLite2-City.mmdb
- **Генерация кодов:** [sqids-go](https://github.com/sqids/sqids-go)
- **Тестирование:** [testify](https://github.com/stretchr/testify) (unit + e2e)
- **Контейнеризация:** Docker + Docker Compose
- **CI/CD:** GitHub Actions (lint, security scan, tests, build)

## API

### POST /api/v1/shorten

Создание короткой ссылки.

**Request:**
```json
{
    "url": "https://example.com/very/long/url"
}
```

**Response (201 Created):**
```json
{
    "short_url": "http://localhost:8080/abc123",
    "short_code": "abc123",
    "management_token": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Ошибки:**
- `400` — неверный формат запроса или URL
- `500` — внутренняя ошибка сервера

### GET /:code

Редирект на оригинальный URL.

- **301 Moved Permanently** — редирект на `Location: <original_url>`
- **404** — код не найден

### GET /api/v1/analytics/:code?token=<management_token>

Получение аналитики по короткой ссылке.

**Response (200 OK):**
```json
{
    "total_clicks": 42,
    "daily_clicks": [
        {"date": "2026-05-30", "count": 15},
        {"date": "2026-05-31", "count": 27}
    ],
    "top_countries": [
        {"country": "US", "count": 20},
        {"country": "RU", "count": 15}
    ],
    "top_referrers": [
        {"referrer": "https://twitter.com", "count": 10},
        {"referrer": "Direct", "count": 32}
    ]
}
```

**Ошибки:**
- `400` — отсутствует short code
- `401` — не указан management token
- `403` — неверный management token

### GET /dashboard

Веб-дашборд для тестирования API (HTML-страница).

## Быстрый старт

### Через Docker Compose (рекомендуется)

```bash
# 1. Клонировать репозиторий
git clone https://github.com/Xindorgi/MnTP_Kursach.git
cd url-shortener

# 2. Скачать GeoIP базу (опционально, для гео-аналитики)
#    Зарегистрироваться на https://www.maxmind.com/ и скачать
#    GeoLite2-City.mmdb в директорию ./geoip/

# 3. Запустить (миграции накатываются автоматически при старте приложения)
docker compose -p url-shortener up --build -d

# 4. Проверить
curl http://localhost:8080/dashboard
```

### Локальный запуск (без Docker)

**Требования:** Go 1.26+, PostgreSQL, Redis

```bash
# 1. Настроить БД
createdb urlshortener
psql -d urlshortener -f migrations/000001_create_urls_table.up.sql
psql -d urlshortener -f migrations/000002_create_url_clicks_table.up.sql
psql -d urlshortener -f migrations/000003_expand_country_column.up.sql

# 2. Скопировать и настроить .env
cp .env.example .env

# 3. Запустить
go run ./cmd/server
```

## Конфигурация

Все настройки задаются через переменные окружения (см. [`.env.example`](.env.example)):

| Переменная | По умолчанию | Описание |
|-----------|-------------|----------|
| `APP_PORT` | `8080` | Порт HTTP-сервера |
| `APP_ENV` | `development` | Окружение |
| `BASE_URL` | `http://localhost:8080` | Базовый URL для коротких ссылок |
| `POSTGRES_HOST` | `localhost` | Хост PostgreSQL |
| `POSTGRES_PORT` | `5432` | Порт PostgreSQL |
| `POSTGRES_USER` | `urlshortener` | Пользователь PostgreSQL |
| `POSTGRES_PASSWORD` | `urlshortener` | Пароль PostgreSQL |
| `POSTGRES_DB` | `urlshortener` | База данных PostgreSQL |
| `POSTGRES_SSLMODE` | `disable` | SSL mode PostgreSQL |
| `REDIS_HOST` | `localhost` | Хост Redis |
| `REDIS_PORT` | `6379` | Порт Redis |
| `REDIS_PASSWORD` | `` | Пароль Redis |
| `GEOIP_DB_PATH` | `./geoip/GeoLite2-City.mmdb` | Путь к GeoIP базе |

## Команды Makefile

| Команда | Описание |
|---------|----------|
| `make build` | Сборка бинарника |
| `make test` | Запуск всех тестов (unit + e2e) |
| `make test-unit` | Unit-тесты (service, worker, transport, repository) |
| `make test-e2e` | E2E-тесты |
| `make test-geoip` | Тесты GeoIP по странам (нужен `geoip/GeoLite2-City.mmdb`) |
| `make test-cover` | Отчёт о покрытии по тестируемым пакетам |
| `make bench` | Бенчмарки |
| `make lint` | Линтинг (golangci-lint) |
| `make sec` | Сканирование безопасности (gosec) |
| `make vulncheck` | Проверка уязвимостей (govulncheck) |
| `make qa` | Все проверки качества (lint → sec → vulncheck → test → bench) |
| `make clean` | Очистка артефактов сборки |

## Миграции

Файлы в `migrations/`:

| Файл | Назначение |
|------|------------|
| `000001_create_urls_table.up.sql` | Таблица `urls` |
| `000002_create_url_clicks_table.up.sql` | Таблица `url_clicks` |
| `000003_expand_country_column.up.sql` | Расширение `country` до `VARCHAR(16)` (значение `LOCAL`) |

### Docker Compose

Миграции накатываются **автоматически** при каждом запуске приложения. Встроенный мигратор ([`internal/migrator`](internal/migrator/migrator.go)):
1. Создаёт таблицу `schema_migrations` для отслеживания применённых миграций
2. Сравнивает файлы из `migrations/` с уже выполненными
3. Применяет новые миграции по одной в отдельных транзакциях

Проверить, что таблицы созданы:

```bash
docker exec url-shortener-db psql -U urlshortener -d urlshortener -c "\dt"
```

Посмотреть, какие миграции уже применены:

```bash
docker exec url-shortener-db psql -U urlshortener -d urlshortener -c "SELECT * FROM schema_migrations"
```

### Локально (без Docker)

```bash
# Применить миграции
psql -d urlshortener -f migrations/000001_create_urls_table.up.sql
psql -d urlshortener -f migrations/000002_create_url_clicks_table.up.sql
psql -d urlshortener -f migrations/000003_expand_country_column.up.sql

# Откатить миграции
psql -d urlshortener -f migrations/000003_expand_country_column.down.sql
psql -d urlshortener -f migrations/000002_create_url_clicks_table.down.sql
psql -d urlshortener -f migrations/000001_create_urls_table.down.sql
```

## Структура проекта

```
./
├── cmd/                                # Точки входа
│   └── server/
│       └── main.go                     # Запуск сервера, DI, graceful shutdown
├── internal/                           # Внутренняя логика приложения
│   ├── config/
│   │   └── config.go                   # Загрузка конфигурации из env
│   ├── domain/                         # Модели данных (сущности)
│   │   ├── url.go                      # URL-сущность
│   │   └── click.go                    # ClickEvent, ClickStats, агрегаты
│   ├── repository/                     # Интерфейсы доступа к данным
│   │   ├── interfaces.go               # URLRepository, ClickRepository, CacheRepository
│   │   ├── postgres/                   # Реализации на PostgreSQL + in-memory
│   │   │   ├── url_repo.go             # URLRepository (pgx + in-memory)
│   │   │   ├── click_repo.go           # ClickRepository (pgx.Batch)
│   │   │   ├── click_repo_memory_test.go
│   │   │   └── url_repo.go
│   │   └── redis/                      # Реализация кэша на Redis + in-memory
│   │       └── cache_repo.go           # CacheRepository (Redis + map fallback)
│   ├── service/
│   │   ├── url_service.go              # Бизнес-логика: создание, редирект, аналитика
│   │   └── url_service_test.go         # Модульные тесты сервиса
│   ├── transport/                      # HTTP-слой (Fiber v2)
│   │   ├── router.go                   # Настройка маршрутов и middleware
│   │   ├── clientip/                   # Утилита извлечения IP клиента
│   │   │   ├── clientip.go
│   │   │   └── clientip_test.go
│   │   ├── handlers/                   # HTTP-хендлеры
│   │   │   ├── shorten.go              # POST /api/v1/shorten
│   │   │   ├── redirect.go             # GET /:code
│   │   │   ├── analytics.go            # GET /api/v1/analytics/:code
│   │   │   ├── dashboard.go            # GET /dashboard
│   │   │   ├── index.go                # GET /
│   │   │   └── templates/              # HTML-шаблоны (embedded)
│   │   │       ├── dashboard.html
│   │   │       └── index.html
│   │   └── middleware/
│   │       └── middleware.go           # Логирование запросов
│   ├── worker/                         # Фоновые обработчики
│   │   ├── analytics_worker.go         # Батчевая запись кликов + GeoIP
│   │   ├── analytics_worker_test.go
│   │   ├── geoip_countries_test.go
│   │   └── geoip_testhelper_test.go
│   ├── migrator/
│   │   └── migrator.go                 # Запуск SQL-миграций
│   └── test_e2e/                       # Интеграционные/E2E-тесты
│       ├── e2e_test.go                 # Основной E2E-тест
│       └── geoip_e2e_test.go           # E2E-тест GeoIP
├── migrations/                         # SQL-миграции
│   ├── 000001_create_urls_table.up.sql
│   ├── 000001_create_urls_table.down.sql
│   ├── 000002_create_url_clicks_table.up.sql
│   ├── 000002_create_url_clicks_table.down.sql
│   ├── 000003_expand_country_column.up.sql
│   └── 000003_expand_country_column.down.sql
├── geoip/                              # GeoLite2 база (скачивается отдельно)
│   └── README.md
├── api/
│   └── openapi.yaml                    # OpenAPI-спецификация
├── templates/                          # Дубликаты шаблонов (для тестов)
│   └── dashboard.html
├── plans/                              # Документация планирования
│   └── architecture-plan.md
├── docs/                               # Техническая документация
│   └── API.md
├── .env.example                        # Пример переменных окружения
├── .golangci.yml                       # Конфигурация линтера
├── docker-compose.yml                  # Docker Compose (PostgreSQL + Redis + app)
├── Dockerfile                          # Многостадийная сборка
├── go.mod / go.sum                     # Модульные зависимости
├── Makefile                            # Автоматизация сборки и тестов
└── README.md

```

## Тестирование

| Пакет | Что проверяется |
|-------|-----------------|
| `internal/service` | Создание ссылки, кэш, resolve, ошибки БД |
| `internal/worker` | Приватные IP → `LOCAL`, GeoIP по публичным IP (US, DE, GB, JP, FR) |
| `internal/transport/clientip` | `X-Forwarded-For` и fallback на `RemoteAddr` |
| `internal/repository/postgres` | Агрегация кликов по странам (in-memory) |
| `internal/test_e2e` | HTTP: shorten → redirect → analytics; GeoIP end-to-end |

Тесты GeoIP (`TestEnrichWithGeoIP_PublicCountries`, `TestE2E_GeoIPCountries`) **пропускаются**, если нет файла `geoip/GeoLite2-City.mmdb` (см. [geoip/README.md](geoip/README.md)). В CI они тоже skip — для локальной проверки стран скачайте базу MaxMind.

```bash
# Все тесты (GeoIP-тесты skip без .mmdb)
make test

# Unit по слоям
make test-unit

# E2E (полный HTTP-цикл)
make test-e2e

# Только GeoIP (США, Германия, UK, Япония, Франция + LOCAL)
make test-geoip

# Покрытие
make test-cover

# Бенчмарки
make bench
```

Симуляция клика из другой страны вручную (как в production за reverse proxy):

```bash
curl -L -H "X-Forwarded-For: 8.8.8.8" "http://localhost:8080/YOUR_CODE"
curl -L -H "X-Forwarded-For: 178.63.41.15" "http://localhost:8080/YOUR_CODE"   # DE
```

## Безопасность

- **Management Token** — UUIDv4, генерируется для каждой ссылки, требуется для доступа к аналитике
- **Graceful Shutdown** — корректное завершение с дообработкой очереди событий
- **Distroless образ** — минимальный Docker-образ без shell и утилит
- **Non-root пользователь** — контейнер запускается от непривилегированного пользователя
- **CORS** — настроен для кросс-доменных запросов
- **Лимит тела запроса** — Fiber по умолчанию ограничивает размер тела (4MB)
- **Валидация URL** — проверка формата и схемы (http/https) перед сохранением

## CI/CD

GitHub Actions автоматически:
1. Запускает линтинг (`golangci-lint`)
2. Сканирует уязвимости (`gosec`, `govulncheck`)
3. Запускает тесты
4. Собирает Docker-образ

## Лицензия

MIT
