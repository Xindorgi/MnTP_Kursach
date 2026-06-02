# API Documentation

## Обзор

Базовый URL: `http://localhost:8080`

Формат ответа: JSON (за исключением `/dashboard` и `/`, которые возвращают HTML).

Аутентификация: для доступа к аналитике требуется `management_token`, который выдаётся при создании короткой ссылки. Токен передаётся как query-параметр `token`.

## Эндпоинты

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

| Код | Описание |
|-----|----------|
| 400 | Неверный формат запроса или URL |
| 500 | Внутренняя ошибка сервера |

**Примеры:**

```bash
curl -X POST http://localhost:8080/api/v1/shorten \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/very/long/url"}'
```

```bash
# Ответ:
# {"short_url":"http://localhost:8080/abc123","short_code":"abc123","management_token":"550e8400-e29b-41d4-a716-446655440000"}
```

---

### GET /:code

Редирект на оригинальный URL.

**Параметры пути:**

| Параметр | Тип | Описание |
|----------|-----|----------|
| `code` | string | Короткий код ссылки (мин. 6 символов) |

**Ответ:**

- **301 Moved Permanently** — редирект на `Location: <original_url>`
- **404** — код не найден

**Примеры:**

```bash
curl -L http://localhost:8080/abc123
```

```bash
# С симуляцией клика из другой страны (через X-Forwarded-For):
curl -L -H "X-Forwarded-For: 8.8.8.8" http://localhost:8080/abc123
curl -L -H "X-Forwarded-For: 178.63.41.15" http://localhost:8080/abc123   # DE
```

---

### GET /api/v1/analytics/:code

Получение аналитики по короткой ссылке.

**Параметры пути:**

| Параметр | Тип | Описание |
|----------|-----|----------|
| `code` | string | Короткий код ссылки (мин. 6 символов) |

**Query-параметры:**

| Параметр | Тип | Обязательный | Описание |
|----------|-----|-------------|----------|
| `token` | UUID | Да | Management token для авторизации |

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

| Код | Описание |
|-----|----------|
| 400 | Отсутствует short code |
| 401 | Не указан management token |
| 403 | Неверный management token или ссылка не найдена |

**Примеры:**

```bash
curl "http://localhost:8080/api/v1/analytics/abc123?token=550e8400-e29b-41d4-a716-446655440000"
```

---

### GET /dashboard

Веб-дашборд для тестирования API (HTML-страница).

**Response (200 OK):** HTML-страница с интерфейсом для:
- Создания коротких ссылок
- Просмотра аналитики
- Тестирования редиректов

```bash
curl http://localhost:8080/dashboard
```

---

### GET /

Главная страница (HTML).

**Response (200 OK):** HTML-страница с приветствием и ссылкой на дашборд.

```bash
curl http://localhost:8080/
```

## Модели данных

### ShortenRequest

Запрос на создание короткой ссылки.

| Поле | Тип | Обязательный | Описание |
|------|-----|-------------|----------|
| `url` | string (uri) | Да | Оригинальный URL для сокращения |

### ShortenResponse

Ответ с созданной короткой ссылкой.

| Поле | Тип | Описание |
|------|-----|----------|
| `short_url` | string (uri) | Полная короткая ссылка для редиректа |
| `short_code` | string | Уникальный короткий код (мин. 6 символов) |
| `management_token` | string (uuid) | Токен для управления ссылкой и доступа к аналитике |

### AnalyticsResponse

Агрегированная статистика переходов.

| Поле | Тип | Описание |
|------|-----|----------|
| `total_clicks` | integer | Общее количество переходов |
| `daily_clicks` | array | Разбивка кликов по дням (последние 30 дней) |
| `top_countries` | array | Топ стран по количеству переходов (до 10) |
| `top_referrers` | array | Топ источников переходов (до 10) |

### DailyClickCount

| Поле | Тип | Описание |
|------|-----|----------|
| `date` | string (date) | Дата в формате YYYY-MM-DD |
| `count` | integer | Количество кликов |

### CountryCount

| Поле | Тип | Описание |
|------|-----|----------|
| `country` | string | Код страны (ISO 3166-1 alpha-2) |
| `count` | integer | Количество кликов |

### ReferrerCount

| Поле | Тип | Описание |
|------|-----|----------|
| `referrer` | string | URL источника перехода или "Direct" для прямых переходов |
| `count` | integer | Количество кликов |

### ErrorResponse

| Поле | Тип | Описание |
|------|-----|----------|
| `error` | string | Описание ошибки |

## Коды ошибок HTTP

| Код | Описание |
|-----|----------|
| 200 | Успешный запрос |
| 201 | Ресурс успешно создан |
| 301 | Постоянное перенаправление |
| 400 | Неверный запрос (невалидные данные) |
| 401 | Не авторизован (отсутствует token) |
| 403 | Доступ запрещён (неверный token) |
| 404 | Ресурс не найден |
| 500 | Внутренняя ошибка сервера |

## OpenAPI спецификация

Полная OpenAPI 3.1 спецификация доступна в файле [`api/openapi.yaml`](../api/openapi.yaml).

Для просмотра в Swagger UI:

```bash
# Установить swagger-ui (через Docker)
docker run -p 8081:8080 -e SWAGGER_JSON=/api/openapi.yaml -v $(pwd)/api:/api swaggerapi/swagger-ui
```

## Формат коротких кодов

Коды генерируются алгоритмом [Sqids](https://github.com/sqids/sqids-go) (на основе Hashids) из числового ID записи в БД:

1. При создании ссылки в БД вставляется запись с `long_url`
2. PostgreSQL возвращает сгенерированный `BIGSERIAL id`
3. Числовой ID кодируется в короткую строку через `sqids.Encode(id)`
4. Короткий код сохраняется в БД и кэшируется в Redis

Минимальная длина кода — 6 символов. Благодаря Sqids, коды не содержат ненормативной лексики и устойчивы к коллизиям.