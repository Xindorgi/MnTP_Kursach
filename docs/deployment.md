# Развёртывание и конфигурация

## Содержание

1. [Конфигурация](#конфигурация)
2. [Docker Compose (рекомендуемый способ)](#docker-compose-рекомендуемый-способ)
3. [Локальный запуск](#локальный-запуск)
4. [Сборка из исходников](#сборка-из-исходников)
5. [Миграции БД](#миграции-бд)
6. [GeoIP база](#geoip-база)
7. [Производственное развёртывание](#производственное-развёртывание)
8. [Мониторинг и логирование](#мониторинг-и-логирование)

---

## Конфигурация

Все настройки задаются через переменные окружения. Пример в файле [`.env.example`](../.env.example).

### Полный список переменных

| Переменная | По умолчанию | Описание |
|-----------|-------------|----------|
| `APP_PORT` | `8080` | Порт HTTP-сервера |
| `APP_ENV` | `development` | Окружение (development/production) |
| `BASE_URL` | `http://localhost:8080` | Базовый URL для формирования коротких ссылок |
| `POSTGRES_HOST` | `localhost` | Хост PostgreSQL |
| `POSTGRES_PORT` | `5432` | Порт PostgreSQL |
| `POSTGRES_USER` | `urlshortener` | Пользователь PostgreSQL |
| `POSTGRES_PASSWORD` | `urlshortener` | Пароль PostgreSQL |
| `POSTGRES_DB` | `urlshortener` | База данных PostgreSQL |
| `POSTGRES_SSLMODE` | `disable` | SSL mode PostgreSQL |
| `REDIS_HOST` | `localhost` | Хост Redis |
| `REDIS_PORT` | `6379` | Порт Redis |
| `REDIS_PASSWORD` | `` | Пароль Redis |
| `GEOIP_DB_PATH` | `./geoip/GeoLite2-City.mmdb` | Путь к GeoIP базе MaxMind |

### Пример `.env` файла

```env
# Application
APP_PORT=8080
APP_ENV=development
BASE_URL=http://localhost:8080

# PostgreSQL
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=urlshortener
POSTGRES_PASSWORD=urlshortener
POSTGRES_DB=urlshortener
POSTGRES_SSLMODE=disable

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=

# GeoIP
GEOIP_DB_PATH=./geoip/GeoLite2-City.mmdb
```

---

## Docker Compose (рекомендуемый способ)

Полная инфраструктура запускается одной командой.

### Состав сервисов

| Сервис | Образ | Назначение |
|--------|-------|------------|
| `postgres` | postgres:16-alpine | Основная БД |
| `redis` | redis:7-alpine | Кэш |
| `app` | сборка из Dockerfile | Приложение |

### Запуск

```bash
# 1. Клонировать репозиторий
git clone https://github.com/Xindorgi/MnTP_Kursach.git
cd url-shortener

# 2. (Опционально) Скачать GeoIP базу
#    Зарегистрироваться на https://www.maxmind.com/
#    Скачать GeoLite2-City.mmdb в ./geoip/

# 3. Запустить все сервисы в фоне
docker compose up -d

# 4. Проверить статус
docker compose ps

# 5. Проверить логи
docker compose logs -f app

# 6. Открыть дашборд
curl http://localhost:8080/dashboard
```

### Остановка

```bash
# Остановить сервисы
docker compose down

# Остановить и удалить volumes (потеря данных!)
docker compose down -v
```

### Переменные окружения для Docker Compose

Можно переопределить любую переменную через `.env` файл в корне проекта или через `docker compose run -e`:

```bash
# Пример: смена порта приложения
APP_PORT=9090 docker compose up -d

# Пример: использование внешней БД
POSTGRES_HOST=192.168.1.100 docker compose up -d
```

### Healthchecks

Docker Compose настроен с healthcheck для PostgreSQL и Redis. Приложение (`app`) запускается только после того, как оба сервиса стали здоровы.

```yaml
depends_on:
  postgres:
    condition: service_healthy
  redis:
    condition: service_healthy
```

---

## Локальный запуск

### Требования

- Go 1.26+
- PostgreSQL 16+
- Redis 7+

### Пошаговая инструкция

```bash
# 1. Создать БД
createdb urlshortener

# 2. Применить миграции вручную
psql -d urlshortener -f migrations/000001_create_urls_table.up.sql
psql -d urlshortener -f migrations/000002_create_url_clicks_table.up.sql
psql -d urlshortener -f migrations/000003_expand_country_column.up.sql

# 3. Настроить конфигурацию
cp .env.example .env
# Отредактировать .env при необходимости

# 4. Запустить приложение
go run ./cmd/server

# 5. Проверить
curl http://localhost:8080/dashboard
```

### In-memory режим (без внешних зависимостей)

Если PostgreSQL или Redis недоступны, приложение автоматически переключается на in-memory реализации:

```bash
# Запуск без PostgreSQL и Redis
# Приложение будет работать, но данные не сохранятся после перезапуска
go run ./cmd/server
```

В логах появятся предупреждения:

```
WARNING: PostgreSQL not available, using in-memory fallback: ...
WARNING: Redis not available, using in-memory cache fallback: ...
```

---

## Сборка из исходников

### Сборка бинарника

```bash
# Сборка с оптимизациями
make build

# Или вручную:
CGO_ENABLED=0 go build -ldflags="-s -w" -o url-shortener ./cmd/server
```

### Docker-образ

```bash
# Сборка Docker-образа
docker build -t url-shortener:latest .

# Запуск с пробросом порта
docker run -p 8080:8080 \
  -e POSTGRES_HOST=host.docker.internal \
  -e REDIS_HOST=host.docker.internal \
  url-shortener:latest
```

### Многостадийная сборка

Dockerfile использует многостадийную сборку:

1. **Stage 1 (builder):** `golang:1.26-alpine` — установка зависимостей, сборка статического бинарника
2. **Stage 2 (runtime):** `gcr.io/distroless/static-debian12:nonroot` — минимальный образ без shell

```dockerfile
# Пример сборки
FROM golang:1.26-alpine AS builder
# ... сборка ...

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /build/server .
USER 65532:65532
ENTRYPOINT ["/app/server"]
```

---

## Миграции БД

### Автоматические (рекомендуется)

При запуске через Docker Compose миграции накатываются автоматически встроенным мигратором ([`internal/migrator`](../internal/migrator/migrator.go)):

1. Создаёт таблицу `schema_migrations` для отслеживания применённых миграций
2. Сравнивает файлы из `migrations/` с уже выполненными
3. Применяет новые миграции по одной в отдельных транзакциях

### Ручные

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

### Проверка статуса

```bash
# Через Docker
docker exec url-shortener-db psql -U urlshortener -d urlshortener -c "\dt"
docker exec url-shortener-db psql -U urlshortener -d urlshortener -c "SELECT * FROM schema_migrations"

# Локально
psql -d urlshortener -c "\dt"
psql -d urlshortener -c "SELECT * FROM schema_migrations"
```

### Список миграций

| Файл | Назначение |
|------|------------|
| `000001_create_urls_table.up.sql` | Таблица `urls` |
| `000002_create_url_clicks_table.up.sql` | Таблица `url_clicks` |
| `000003_expand_country_column.up.sql` | Расширение `country` до `VARCHAR(16)` (значение `LOCAL`) |

---

## GeoIP база

### Получение

1. Зарегистрироваться на [MaxMind](https://www.maxmind.com/)
2. Скачать **GeoLite2-City** базу в формате `.mmdb`
3. Поместить файл в `./geoip/GeoLite2-City.mmdb`

### Настройка пути

По умолчанию приложение ищет базу по пути `./geoip/GeoLite2-City.mmdb`. Можно изменить через переменную `GEOIP_DB_PATH`:

```bash
GEOIP_DB_PATH=/data/GeoLite2-City.mmdb go run ./cmd/server
```

### Без GeoIP

Если файл не найден, приложение запускается без GeoIP-обогащения. Все клики будут помечены как `LOCAL`:

```
WARNING: Failed to open GeoIP database at ./geoip/GeoLite2-City.mmdb: ... GeoIP disabled.
```

### В Docker

GeoIP база монтируется как read-only volume:

```yaml
volumes:
  - ./geoip:/app/geoip:ro
```

---

## Производственное развёртывание

### Рекомендации

1. **Использовать reverse proxy** — Nginx или Caddy перед приложением для:
   - TLS/SSL терминирования
   - Rate limiting
   - Настройки корректного `X-Forwarded-For`

2. **Настроить production-переменные:**
   ```env
   APP_ENV=production
   BASE_URL=https://your-domain.com
   POSTGRES_SSLMODE=require
   REDIS_PASSWORD=strong-password
   ```

3. **Регулярное резервное копирование БД:**
   ```bash
   docker exec url-shortener-db pg_dump -U urlshortener urlshortener > backup_$(date +%Y%m%d).sql
   ```

4. **Мониторинг** — настроить сбор метрик и алертинг

### Пример docker-compose для production

```yaml
version: "3.8"

services:
  postgres:
    image: postgres:16-alpine
    restart: always
    environment:
      POSTGRES_USER: urlshortener
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?required}
      POSTGRES_DB: urlshortener
    volumes:
      - postgres_data:/var/lib/postgresql/data
    networks:
      - internal

  redis:
    image: redis:7-alpine
    restart: always
    command: redis-server --requirepass ${REDIS_PASSWORD:?required}
    volumes:
      - redis_data:/data
    networks:
      - internal

  app:
    build: .
    restart: always
    ports:
      - "127.0.0.1:8080:8080"  # Только localhost, reverse proxy спереди
    environment:
      APP_ENV: production
      BASE_URL: https://your-domain.com
      POSTGRES_HOST: postgres
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?required}
      REDIS_HOST: redis
      REDIS_PASSWORD: ${REDIS_PASSWORD:?required}
      POSTGRES_SSLMODE: require
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    networks:
      - internal

  nginx:
    image: nginx:alpine
    restart: always
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./ssl:/etc/nginx/ssl:ro
    depends_on:
      - app
    networks:
      - internal

volumes:
  postgres_data:
  redis_data:

networks:
  internal:
    driver: bridge
```

---

## Мониторинг и логирование

### Логи приложения

Приложение использует стандартный `log` пакет Go. Логи выводятся в stdout/stderr:

```
2026/06/02 12:00:00 Server starting on :8080
2026/06/02 12:00:01 [POST] /api/v1/shorten - 201 (12.345ms)
2026/06/02 12:00:02 [GET] /abc123 - 301 (1.234ms)
2026/06/02 12:00:03 Flushed 50 click events to database
```

### Healthcheck endpoints

Для мониторинга можно добавить эндпоинт `/health` (не реализован в текущей версии, но может быть добавлен):

```go
app.Get("/health", func(c *fiber.Ctx) error {
    return c.JSON(fiber.Map{
        "status": "ok",
        "time":   time.Now(),
    })
})
```

### Метрики

Для production рекомендуется добавить Prometheus метрики через [fiber-prometheus](https://github.com/ansrivas/fiber-prometheus):

```go
import "github.com/ansrivas/fiberprometheus/v2"

prometheus := fiberprometheus.New("url-shortener")
prometheus.RegisterAt(app, "/metrics")
app.Use(prometheus.Middleware)
```

---

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