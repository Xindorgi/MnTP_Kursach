# URL Shortener

Высокопроизводительный сервис сокращения ссылок с аналитикой на Go.

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
git clone https://github.com/v8950/url-shortener.git
cd url-shortener

# 2. Скачать GeoIP базу (опционально, для гео-аналитики)
#    Зарегистрироваться на https://www.maxmind.com/ и скачать
#    GeoLite2-City.mmdb в директорию ./geoip/

# 3. Запустить
docker compose up -d

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
| `make test-unit` | Unit-тесты сервисного слоя |
| `make test-e2e` | E2E-тесты |
| `make bench` | Бенчмарки |
| `make lint` | Линтинг (golangci-lint) |
| `make sec` | Сканирование безопасности (gosec) |
| `make vulncheck` | Проверка уязвимостей (govulncheck) |
| `make qa` | Все проверки качества (lint → sec → vulncheck → test → bench) |
| `make clean` | Очистка артефактов сборки |

## Миграции

```bash
# Применить миграции
psql -d urlshortener -f migrations/000001_create_urls_table.up.sql
psql -d urlshortener -f migrations/000002_create_url_clicks_table.up.sql

# Откатить миграции
psql -d urlshortener -f migrations/000001_create_urls_table.down.sql
psql -d urlshortener -f migrations/000002_create_url_clicks_table.down.sql
```

## Структура проекта

```
.
├── .github/workflows/       # CI/CD (GitHub Actions)
├── api/                     # OpenAPI спецификация
├── cmd/server/              # Точка входа
├── internal/
│   ├── config/              # Конфигурация
│   ├── domain/              # Модели данных (URL, ClickEvent, ClickStats)
│   ├── repository/
│   │   ├── interfaces.go    # Интерфейсы репозиториев
│   │   ├── postgres/        # Реализация PostgreSQL + in-memory fallback
│   │   └── redis/           # Реализация Redis + in-memory fallback
│   ├── service/             # Бизнес-логика
│   ├── test_e2e/            # E2E-тесты
│   ├── transport/
│   │   ├── handlers/        # HTTP-хендлеры
│   │   │   └── templates/   # HTML-шаблоны
│   │   ├── middleware/      # Middleware (логирование)
│   │   └── router.go        # Маршрутизация
│   └── worker/              # Analytics Worker (GeoIP + batch insert)
├── migrations/              # SQL-миграции
├── templates/               # HTML-шаблоны (для Docker)
├── docker-compose.yml       # Инфраструктура
├── Dockerfile               # Многостадийная сборка
├── Makefile                 # Автоматизация
└── .env.example             # Пример конфигурации
```

## Тестирование

Проект покрыт unit-тестами (слой service) и e2e-тестами (полный HTTP-цикл).

```bash
# Все тесты
make test

# Только unit
make test-unit

# Только e2e
make test-e2e

# Бенчмарки
make bench
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