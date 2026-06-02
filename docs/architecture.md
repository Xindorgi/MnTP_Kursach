# Архитектура проекта

## Общая архитектура (Big Picture)

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

## Диаграмма компонентов и зависимостей

```mermaid
graph TD
    subgraph "Transport Layer"
        H[Handlers]
        M[Middlewares<br/>Logger, Recovery, CORS]
        T[Templates<br/>Dashboard HTML]
    end

    subgraph "Service Layer"
        US[URL Service<br/>CreateShortURL, ResolveURL,<br/>GetAnalytics, RecordClick]
        AW[Analytics Worker<br/>GeoIP lookup + Batch insert]
        CH[chan ClickEvent<br/>buffer: 1000]
    end

    subgraph "Repository Layer"
        UR[URL Repository<br/>PostgreSQL + in-memory]
        CR[Click Repository<br/>PostgreSQL + in-memory]
        CAR[Cache Repository<br/>Redis + in-memory]
    end

    subgraph "External"
        PG[(PostgreSQL 16)]
        RD[(Redis 7)]
    end

    H --> US
    US --> UR
    US --> CAR
    US --> CH
    CH --> AW
    AW --> CR
    UR --> PG
    CR --> PG
    CAR --> RD
    T --> US

    style CH fill:#f9f,stroke:#333,stroke-width:2px
    style AW fill:#bbf,stroke:#333,stroke-width:2px
```

## Схема базы данных

```mermaid
erDiagram
    urls {
        bigint id PK "BIGSERIAL"
        text long_url "Оригинальный URL"
        varchar short_code "Короткий код (уникальный)"
        uuid management_token "Токен для доступа к аналитике"
        timestamptz created_at "Дата создания"
        timestamptz updated_at "Дата обновления"
    }

    url_clicks {
        bigint id PK "BIGSERIAL"
        bigint url_id FK "Ссылка на urls.id"
        varchar ip_address "IP адрес клиента"
        text user_agent "User-Agent браузера"
        text referer "Referer (источник перехода)"
        varchar country "Код страны (ISO 3166-1 alpha-2)"
        varchar city "Название города"
        timestamptz clicked_at "Время клика"
    }

    schema_migrations {
        text filename PK "Имя файла миграции"
        timestamptz applied_at "Дата применения"
    }

    urls ||--o{ url_clicks : "has"
```

### Таблица `urls`

```sql
CREATE TABLE urls (
    id               BIGSERIAL    PRIMARY KEY,
    long_url         TEXT         NOT NULL,
    short_code       VARCHAR(10)  UNIQUE,
    management_token UUID         NOT NULL DEFAULT gen_random_uuid(),
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_urls_short_code ON urls(short_code);
```

### Таблица `url_clicks`

```sql
CREATE TABLE url_clicks (
    id          BIGSERIAL    PRIMARY KEY,
    url_id      BIGINT       NOT NULL REFERENCES urls(id) ON DELETE CASCADE,
    ip_address  VARCHAR(45),
    user_agent  TEXT,
    referer     TEXT,
    country     VARCHAR(16),  -- ISO 3166-1 alpha-2 (RU, US, DE...) или 'LOCAL'
    city        VARCHAR(100),
    clicked_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_url_clicks_url_id ON url_clicks(url_id);
CREATE INDEX idx_url_clicks_clicked_at ON url_clicks(clicked_at);
```

### Таблица `schema_migrations`

```sql
CREATE TABLE schema_migrations (
    filename    TEXT         PRIMARY KEY,
    applied_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
```

## Диаграмма последовательности: создание ссылки

```mermaid
sequenceDiagram
    participant Client
    participant Fiber as Fiber Handler
    participant Service as URL Service
    participant Cache as Redis Cache
    participant DB as PostgreSQL

    Note over Client,DB: Создание короткой ссылки
    Client->>Fiber: POST /api/v1/shorten {url: "https://..."}
    Fiber->>Fiber: Валидация URL (http/https)
    Fiber->>Service: CreateShortURL(longURL)
    Service->>DB: INSERT (long_url) RETURNING id, management_token
    DB-->>Service: id=42, token=uuid
    Service->>Service: sqids.Encode(42) → "abc123"
    Service->>DB: UPDATE short_code='abc123' WHERE id=42
    Service->>Cache: SET("abc123", longURL, TTL=24h)
    Service-->>Fiber: URL{shortCode, managementToken}
    Fiber-->>Client: 201 {short_url, short_code, management_token}
```

## Диаграмма последовательности: редирект + аналитика

```mermaid
sequenceDiagram
    participant Client
    participant Fiber as Fiber Handler
    participant Service as URL Service
    participant Cache as Redis Cache
    participant DB as PostgreSQL
    participant Worker as Analytics Worker

    Note over Client,Worker: Редирект по короткой ссылке
    Client->>Fiber: GET /abc123
    Fiber->>Service: ResolveURL("abc123")
    Service->>Cache: GET("abc123")
    alt Cache hit
        Cache-->>Service: longURL
    else Cache miss
        Service->>DB: SELECT long_url FROM urls WHERE short_code='abc123'
        DB-->>Service: longURL
        Service->>Cache: SET("abc123", longURL) (best-effort)
    end
    Service-->>Fiber: longURL
    Fiber->>Service: RecordClickByID(urlID, ip, ua, referer)
    Service->>Worker: chan <- ClickEvent{urlID, IP, UA, Referer}
    Fiber-->>Client: 301 Redirect → longURL

    Note over Client,Worker: Асинхронная запись аналитики (фоновая горутина)
    Worker->>Worker: enrichWithGeoIP(&event)
    alt Private IP (127.0.0.1, 10.x.x.x, etc.)
        Worker->>Worker: country = "LOCAL"
    else Public IP
        Worker->>Worker: geoIP.City(ip) → country, city
    end
    Worker->>Worker: Накопление батча (до 50 событий)
    alt Batch full (50 events)
        Worker->>DB: Batch INSERT INTO url_clicks
    else FlushInterval (1 sec)
        Worker->>DB: Batch INSERT INTO url_clicks
    end
```

## Диаграмма последовательности: получение аналитики

```mermaid
sequenceDiagram
    participant Client
    participant Fiber as Fiber Handler
    participant Service as URL Service
    participant DB as PostgreSQL

    Note over Client,DB: Получение аналитики
    Client->>Fiber: GET /api/v1/analytics/abc123?token=uuid
    Fiber->>Service: GetAnalytics("abc123", "uuid")
    Service->>DB: SELECT * FROM urls WHERE short_code='abc123'
    DB-->>Service: URL{id=42, management_token=uuid}
    Service->>Service: Проверка management_token
    Service->>DB: SELECT COUNT(*) FROM url_clicks WHERE url_id=42
    DB-->>Service: total_clicks=42
    Service->>DB: SELECT DATE(clicked_at), COUNT(*) ... GROUP BY DATE ... ORDER BY DATE DESC
    DB-->>Service: daily_clicks[...]
    Service->>DB: SELECT country, COUNT(*) ... GROUP BY country ... ORDER BY count DESC LIMIT 10
    DB-->>Service: top_countries[...]
    Service->>DB: SELECT referer, COUNT(*) ... GROUP BY referer ... ORDER BY count DESC LIMIT 10
    DB-->>Service: top_referrers[...]
    Service-->>Fiber: ClickStats{total, daily, countries, referrers}
    Fiber-->>Client: 200 {total_clicks, daily_clicks, top_countries, top_referrers}
```

## Поток данных: создание короткой ссылки

```
Client → POST /api/v1/shorten { "url": "https://..." }
  → Handler валидирует URL (схема http/https)
  → Service.CreateShortURL()
    → Repository.Insert(longURL) → получает id + management_token
    → sqids.Encode(id) → shortCode
    → Repository.UpdateShortCode(id, shortCode)
    → CacheRepository.Set(shortCode, longURL, TTL=24h)
  ← Response 201 { "short_url": "http://localhost:8080/abc123", "management_token": "uuid" }
```

## Поток данных: редирект по короткой ссылке

```
Client → GET /abc123
  → Handler извлекает shortCode из пути
  → Service.ResolveURL(shortCode)
    → CacheRepository.Get(shortCode) → если есть, сразу отдаём
    → Repository.FindByShortCode(shortCode) → longURL
    → CacheRepository.Set(shortCode, longURL) — кэшируем (best-effort)
  → Отправляем ClickEvent в канал (асинхронно, не блокируя ответ)
  → Редирект 301 на longURL
```

## Поток данных: асинхронная запись аналитики

```
ClickEvent { URLID, IP, UserAgent, Referer, Timestamp }
  → отправляется в buffered chan ClickEvent (capacity 1000)
  → AnalyticsWorker (отдельная горутина) читает из канала
  → enrichWithGeoIP(&event):
    - Если IP пустой → country = "LOCAL"
    - Если IP приватный (127.0.0.1, 10.x.x.x, 172.16-31.x.x, 192.168.x.x) → country = "LOCAL"
    - Если GeoIP база загружена → geoIP.City(ip) → country + city
  → Накопление батча (до 50 событий)
  → При достижении BatchSize (50) или по таймеру (1 сек):
    → BatchInsert в url_clicks (pgx.Batch)
```

## Схема слоёв приложения

```mermaid
graph TD
    subgraph "cmd/server/main.go"
        DI[Dependency Injection<br/>Config → Repos → Worker → Service → Handlers → Router]
    end

    subgraph "internal/config"
        C[Config<br/>Загрузка из env-переменных]
    end

    subgraph "internal/domain"
        U[URL model]
        CE[ClickEvent model]
        CS[ClickStats model]
    end

    subgraph "internal/repository"
        I[Interfaces<br/>URLRepository<br/>ClickRepository<br/>CacheRepository]
        PG[PostgreSQL реализация<br/>+ In-memory fallback]
        RD[Redis реализация<br/>+ In-memory fallback]
    end

    subgraph "internal/service"
        S[URL Service<br/>Бизнес-логика]
    end

    subgraph "internal/worker"
        W[Analytics Worker<br/>GeoIP + Batch]
    end

    subgraph "internal/transport"
        R[Router<br/>Fiber routes]
        H[Handlers<br/>shorten, redirect, analytics, dashboard]
        M[Middleware<br/>Logger]
        T[Templates<br/>HTML]
    end

    subgraph "internal/migrator"
        MIG[Migrator<br/>Автоматические миграции]
    end

    DI --> C
    DI --> PG
    DI --> RD
    DI --> W
    DI --> S
    DI --> R
    S --> I
    S --> W
    R --> H
    H --> S
    PG --> I
    RD --> I
    MIG --> PG
```

## Обработка ошибок и fallback-механизмы

```mermaid
flowchart LR
    subgraph "Startup"
        START[Запуск приложения]
    end

    subgraph "PostgreSQL"
        PG_TRY[Попытка подключения]
        PG_OK[PostgreSQL OK]
        PG_FAIL[PostgreSQL недоступен]
    end

    subgraph "Redis"
        RD_TRY[Попытка подключения]
        RD_OK[Redis OK]
        RD_FAIL[Redis недоступен]
    end

    subgraph "GeoIP"
        GI_TRY[Загрузка GeoLite2-City.mmdb]
        GI_OK[GeoIP OK]
        GI_FAIL[Файл не найден]
    end

    START --> PG_TRY
    PG_TRY --> PG_OK
    PG_TRY --> PG_FAIL
    PG_FAIL --> IM_URL[InMemoryURLRepository]
    PG_FAIL --> IM_CLICK[InMemoryClickRepository]
    PG_OK --> PG_POOL[pgxpool]
    PG_POOL --> MIG[Миграции БД]

    START --> RD_TRY
    RD_TRY --> RD_OK
    RD_TRY --> RD_FAIL
    RD_FAIL --> IM_CACHE[InMemoryCacheRepository]

    START --> GI_TRY
    GI_TRY --> GI_OK
    GI_TRY --> GI_FAIL
    GI_FAIL --> GEOIP_DISABLED[GeoIP отключён<br/>все IP → LOCAL]

    PG_OK --> RD_TRY
```

## Ключевые архитектурные решения

### 1. Луковая архитектура (Layered Architecture)

Код разделён на слои: `handlers → service → repository`. Каждый слой зависит только от нижележащего через интерфейсы. Это обеспечивает:

- **Тестируемость** — каждый слой можно тестировать изолированно с моками
- **Гибкость** — замена реализации (например, PostgreSQL → in-memory) без изменения вызывающего кода
- **Чёткое разделение ответственности**

### 2. Асинхронная аналитика через каналы

Click-события отправляются в буферизированный канал (capacity 1000) и обрабатываются фоновой горутиной. Это гарантирует:

- **Низкую задержку редиректа** — запись в БД не блокирует ответ клиенту
- **Graceful degradation** — при переполнении канала события дропаются, но сервис продолжает работу
- **Batch-эффективность** — группировка вставок снижает нагрузку на БД

### 3. In-memory fallback

При недоступности PostgreSQL или Redis сервис автоматически переключается на in-memory реализации. Это позволяет:

- Запускать сервис без внешних зависимостей для разработки
- Сохранять работоспособность при сбоях инфраструктуры
- Использовать в тестовых окружениях без БД

### 4. Sqids для генерации кодов

Вместо случайных строк или хэшей используется [Sqids](https://github.com/sqids/sqids-go) (Hashids):

- **Без коллизий** — код детерминированно вычисляется из числового ID
- **Короткие коды** — минимальная длина 6 символов
- **Без ненормативной лексики** — Sqids гарантирует читаемые коды

### 5. Graceful Shutdown

При получении сигнала SIGINT/SIGTERM:

1. Сервер перестаёт принимать новые запросы
2. Analytics Worker дообрабатывает оставшиеся события в канале
3. Все накопленные батчи сбрасываются в БД
4. Ресурсы (GeoIP reader, пулы соединений) корректно закрываются