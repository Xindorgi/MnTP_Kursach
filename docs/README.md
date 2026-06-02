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
psql -d urlshortener -f migrations/000003_expand_country_column.up.sql

# 2. Скопировать и настроить .env
cp .env.example .env

# 3. Запустить
go run ./cmd/server
```

## Структура проекта

```
.
├── .github/workflows/       # CI/CD (GitHub Actions)
├── api/                     # OpenAPI спецификация
├── cmd/server/              # Точка входа
├── docs/                    # Документация
├── internal/
│   ├── config/              # Конфигурация
│   ├── domain/              # Модели данных (URL, ClickEvent, ClickStats)
│   ├── migrator/            # Автоматические миграции БД при старте
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

## Технологический стек

| Компонент | Технология | Назначение |
|-----------|-----------|------------|
| **Язык** | Go 1.26 | Высокая производительность, встроенная конкурентность |
| **HTTP-сервер** | Fiber v2 | Обработка запросов, маршрутизация, middleware |
| **Бизнес-логика** | Service layer | Создание/резолвинг URL, валидация, управление токенами |
| **Хранилище URL** | PostgreSQL + in-memory fallback | Персистентное хранение ссылок |
| **Хранилище кликов** | PostgreSQL + in-memory fallback | Персистентное хранение аналитики |
| **Кэш** | Redis + in-memory fallback | Быстрое разрешение коротких кодов |
| **Analytics Worker** | Go-горутина | Асинхронная обработка кликов: GeoIP → batch insert |
| **Генерация кодов** | Sqids (Hashids) | Короткие уникальные коды из числовых ID |
| **GeoIP** | geoip2-golang + GeoLite2-City.mmdb | Локальное определение гео без внешних API |
| **Драйвер БД** | pgx v5 | Самый производительный Go-драйвер для PostgreSQL |
| **Тестирование** | testify | Unit + E2E тесты |
| **Контейнеризация** | Docker + Docker Compose | Изолированная среда разработки |
| **CI/CD** | GitHub Actions | Lint, security scan, tests, build |

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

## Тестирование

| Пакет | Что проверяется |
|-------|-----------------|
| `internal/service` | Создание ссылки, кэш, resolve, ошибки БД |
| `internal/worker` | Приватные IP → `LOCAL`, GeoIP по публичным IP (US, DE, GB, JP, FR) |
| `internal/transport/clientip` | `X-Forwarded-For` и fallback на `RemoteAddr` |
| `internal/repository/postgres` | Агрегация кликов по странам (in-memory) |
| `internal/test_e2e` | HTTP: shorten → redirect → analytics; GeoIP end-to-end |

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