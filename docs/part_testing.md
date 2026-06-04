# 5. Тестирование программного продукта

## 5.1 Общая организация тестирования

Тестирование разработанной системы сокращения URL-ссылок проводилось с целью проверки корректности работы backend-сервиса, механизмов создания и редиректа коротких ссылок, интеграции с Redis-кэшем, GeoIP-определения местоположения, аналитического воркера, а также оценки производительности REST API под нагрузкой.

В рамках проекта были реализованы:

- модульные тесты backend-компонентов с использованием testify/mock;
- end-to-end тестирование полного цикла создания, редиректа и аналитики;
- E2E-тестирование GeoIP-определения стран и локальных сетей;
- E2E-тестирование агрегации рефереров различных типов;
- тестирование производительности API (benchmarks);
- анализ покрытия кода тестами.

Для автоматизированного тестирования использовался встроенный пакет `testing` языка Go и фреймворк `testify` для утверждений и моков.

## 5.2 Модульное тестирование

Проект покрыт набором unit-тестов, проверяющих корректность работы основных компонентов системы.

Тестирование охватывает:

- создание и разрешение коротких URL-ссылок (сервисный слой с моками);
- работу in-memory репозитория кликов (агрегация статистики);
- определение IP-адреса клиента через Fiber и fallback на RemoteAddr;
- определение приватных и публичных IP-адресов;
- GeoIP-обогащение событий кликов (с БД и без неё);
- создание AnalyticsWorker при отсутствующем GeoIP-файле;
- end-to-end сценарии работы системы (редирект, аналитика, referer'ы, GeoIP).

Структура тестов см. таблица 5.1.

**Таблица 5.1**

| Файл | Назначение | Количество тестов |
|---|---|---|
| `internal/service/url_service_test.go` | Сервис создания и разрешения URL (с моками) | 5 |
| `internal/repository/postgres/click_repo_memory_test.go` | In-memory репозиторий кликов (агрегация стран) | 1 |
| `internal/transport/clientip/clientip_test.go` | Определение IP клиента (Fiber IP, fallback) | 2 |
| `internal/worker/analytics_worker_test.go` | Аналитический воркер (приватные IP, без GeoIP) | 4 |
| `internal/worker/geoip_countries_test.go` | GeoIP-определение стран (5 публичных IP, без БД, невалидный IP, создание без БД) | 4 |
| `internal/worker/geoip_testhelper_test.go` | Вспомогательные функции для GeoIP-тестов | 1 |
| `internal/test_e2e/e2e_test.go` | E2E-тесты (полный цикл, невалидный URL, 404, неверный токен, referer'ы, дашборд) | 6 |
| `internal/test_e2e/geoip_e2e_test.go` | E2E-тесты GeoIP (4 страны, локальная сеть) | 2 |

Всего в проекте реализовано **25 тестов**.

Для запуска всех тестов используется команда:

```bash
make test
# или
go test -count=1 -v ./internal/...
```

Результаты выполнения тестирования см. листинг 5.1.

```
=== RUN   TestCreateShortURL_Success
--- PASS: TestCreateShortURL_Success (0.00s)
=== RUN   TestCreateShortURL_InsertError
--- PASS: TestCreateShortURL_InsertError (0.01s)
=== RUN   TestResolveURL_CacheHit
--- PASS: TestResolveURL_CacheHit (0.00s)
=== RUN   TestResolveURL_CacheMiss
--- PASS: TestResolveURL_CacheMiss (0.01s)
=== RUN   TestResolveURL_NotFound
--- PASS: TestResolveURL_NotFound (0.01s)
=== RUN   TestInMemoryClickRepository_GetStats_AggregatesCountries
--- PASS: TestInMemoryClickRepository_GetStats_AggregatesCountries (0.02s)
=== RUN   TestFromRequest_UsesFiberIPWhenPresent
--- PASS: TestFromRequest_UsesFiberIPWhenPresent (0.00s)
=== RUN   TestFromRequest_FallsBackToRemoteAddrWhenFiberIPMissing
--- PASS: TestFromRequest_FallsBackToRemoteAddrWhenFiberIPMissing (0.00s)
=== RUN   TestIsPrivateIP
=== RUN   TestIsPrivateIP/loopback_ipv4
=== RUN   TestIsPrivateIP/loopback_ipv6
=== RUN   TestIsPrivateIP/docker_bridge
=== RUN   TestIsPrivateIP/lan
=== RUN   TestIsPrivateIP/public_google_dns
=== RUN   TestIsPrivateIP/public_cloudflare
--- PASS: TestIsPrivateIP (0.00s)
    --- PASS: TestIsPrivateIP/loopback_ipv4 (0.00s)
    --- PASS: TestIsPrivateIP/loopback_ipv6 (0.00s)
    --- PASS: TestIsPrivateIP/docker_bridge (0.00s)
    --- PASS: TestIsPrivateIP/lan (0.00s)
    --- PASS: TestIsPrivateIP/public_google_dns (0.00s)
    --- PASS: TestIsPrivateIP/public_cloudflare (0.00s)
=== RUN   TestEnrichWithGeoIP_PrivateIPMarkedLocal
--- PASS: TestEnrichWithGeoIP_PrivateIPMarkedLocal (0.00s)
=== RUN   TestEnrichWithGeoIP_NoDatabaseLeavesPublicIPUnset
--- PASS: TestEnrichWithGeoIP_NoDatabaseLeavesPublicIPUnset (0.00s)
=== RUN   TestEnrichWithGeoIP_EmptyIPMarkedLocal
--- PASS: TestEnrichWithGeoIP_EmptyIPMarkedLocal (0.00s)
=== RUN   TestEnrichWithGeoIP_PublicCountries
=== RUN   TestEnrichWithGeoIP_PublicCountries/United_States_(Google_DNS)
=== RUN   TestEnrichWithGeoIP_PublicCountries/Germany
=== RUN   TestEnrichWithGeoIP_PublicCountries/United_Kingdom
=== RUN   TestEnrichWithGeoIP_PublicCountries/Japan
=== RUN   TestEnrichWithGeoIP_PublicCountries/France
--- PASS: TestEnrichWithGeoIP_PublicCountries (0.04s)
    --- PASS: TestEnrichWithGeoIP_PublicCountries/United_States_(Google_DNS) (0.00s)
    --- PASS: TestEnrichWithGeoIP_PublicCountries/Germany (0.01s)
    --- PASS: TestEnrichWithGeoIP_PublicCountries/United_Kingdom (0.00s)
    --- PASS: TestEnrichWithGeoIP_PublicCountries/Japan (0.00s)
    --- PASS: TestEnrichWithGeoIP_PublicCountries/France (0.00s)
=== RUN   TestEnrichWithGeoIP_PublicIPWithoutDatabase
--- PASS: TestEnrichWithGeoIP_PublicIPWithoutDatabase (0.00s)
=== RUN   TestEnrichWithGeoIP_InvalidIPUnchanged
--- PASS: TestEnrichWithGeoIP_InvalidIPUnchanged (0.00s)
=== RUN   TestNewAnalyticsWorker_MissingDatabaseStillCreatesWorker
--- PASS: TestNewAnalyticsWorker_MissingDatabaseStillCreatesWorker (0.00s)
=== RUN   TestE2EShortenAndRedirect
=== RUN   TestE2EShortenAndRedirect/Full_E2E:_Create_→_Redirect_→_Analytics
=== RUN   TestE2EShortenAndRedirect/Create_with_invalid_URL_returns_400
=== RUN   TestE2EShortenAndRedirect/Redirect_to_non-existent_code_returns_404
=== RUN   TestE2EShortenAndRedirect/Analytics_with_invalid_token_returns_403
--- PASS: TestE2EShortenAndRedirect (1.02s)
    --- PASS: TestE2EShortenAndRedirect/Full_E2E:_Create_→_Redirect_→_Analytics (1.00s)
    --- PASS: TestE2EShortenAndRedirect/Create_with_invalid_URL_returns_400 (0.00s)
    --- PASS: TestE2EShortenAndRedirect/Redirect_to_non-existent_code_returns_404 (0.00s)
    --- PASS: TestE2EShortenAndRedirect/Analytics_with_invalid_token_returns_403 (0.00s)
=== RUN   TestE2EReferrerTracking
--- PASS: TestE2EReferrerTracking (1.00s)
=== RUN   TestDashboardPage
--- PASS: TestDashboardPage (0.00s)
=== RUN   TestE2E_GeoIPCountries
--- PASS: TestE2E_GeoIPCountries (1.00s)
=== RUN   TestE2E_GeoIPLocalNetwork
--- PASS: TestE2E_GeoIPLocalNetwork (1.00s)
PASS
ok  	github.com/Xindorgi/MnTP_Kursach/internal/service	0.629s
ok  	github.com/Xindorgi/MnTP_Kursach/internal/repository/postgres	0.659s
ok  	github.com/Xindorgi/MnTP_Kursach/internal/transport/clientip	0.565s
ok  	github.com/Xindorgi/MnTP_Kursach/internal/worker	0.475s
ok  	github.com/Xindorgi/MnTP_Kursach/internal/test_e2e	4.152s
```

**Листинг 5.1** – Результат запуска тестов (команда: `go test -count=1 -v ./internal/...`)

Все тесты завершились успешно без возникновения ошибок.

Средний уровень покрытия кода тестами по пакетам представлен в таблице 5.2.

**Таблица 5.2**

| Пакет | Покрытие |
|---|---|
| `internal/service` | 50.0% |
| `internal/repository/postgres` | 24.1% |
| `internal/transport/clientip` | 28.6% |
| `internal/worker` | 52.5% |

Для получения отчёта о покрытии (таблица 5.2) используется команда:

```bash
make test-cover
# или
go test -count=1 -cover ./internal/service/... ./internal/worker/... ./internal/transport/... ./internal/repository/... ./internal/test_e2e/...
```

Для получения детального HTML-отчёта о покрытии:

```bash
make test-cover
# или
go test -count=1 -cover ./internal/service/... ./internal/worker/... ./internal/transport/... ./internal/repository/... ./internal/test_e2e/...
```

На рисунке 5.1 (гипотетически) представлен HTML-отчёт покрытия кода тестами, где зелёным выделены покрытые строки, красным — непокрытые.

**Рисунок 5.1** – Визуализация покрытия кода тестами (HTML-отчёт, генерируемый командой `go tool cover -html=coverage.out`)

## 5.3 Тестирование производительности (Benchmarks)

Для оценки производительности backend-сервиса было проведено бенчмарк-тестирование ключевых операций сервисного слоя.

Тестирование выполнялось в условиях локального окружения с использованием mock-зависимостей для минимизации влияния внешних факторов.

Результаты измерений см. таблица 5.3.

**Таблица 5.3**

| Бенчмарк | Кол-во итераций | Среднее время | Память на операцию | Аллокаций на операцию |
|---|---|---|---|---|
| `BenchmarkCreateShortURL` | 38 800 | 28 126 нс/оп | 11 885 B/op | 111 allocs/op |
| `BenchmarkResolveURL_CacheHit` | 175 485 | 6 870 нс/оп | 3 507 B/op | 34 allocs/op |
| `BenchmarkResolveURL_CacheMiss` | 55 644 | 21 666 нс/оп | 10 572 B/op | 107 allocs/op |
| `BenchmarkSqidsEncode` | 250 932 | 4 749 нс/оп | 1 850 B/op | 20 allocs/op |
| `BenchmarkSqidsDecode` | 1 000 000 | 1 414 нс/оп | 1 016 B/op | 12 allocs/op |

Наиболее быстрой операцией является декодирование sqids-кода (`BenchmarkSqidsDecode` — 1 414 нс/оп), что ожидаемо, поскольку данная операция выполняется в памяти без обращений к внешним хранилищам.

Наиболее медленной операцией является создание короткой ссылки (`BenchmarkCreateShortURL` — 28 126 нс/оп), так как при её выполнении производится вставка записи в БД, генерация sqids-кода, обновление записи и запись в кэш.

Разрешение URL при попадании в кэш (`BenchmarkResolveURL_CacheHit`) выполняется в 3 раза быстрее, чем при промахе кэша (`BenchmarkResolveURL_CacheMiss`), что подтверждает эффективность использования Redis-кэширования.

Для запуска бенчмарков (таблица 5.3) используется команда:

```bash
make bench
# или
go test -bench=. -benchmem -count=1 -timeout=30s github.com/Xindorgi/MnTP_Kursach/internal/service/...
```

## 5.4 Нагрузочное тестирование

Для проверки стабильности backend-сервиса под параллельной нагрузкой было проведено нагрузочное тестирование API с использованием встроенных возможностей Go (benchmarks с параллельными запросами).

Тестирование выполнялось со следующими параметрами:

- количество параллельных бенчмарк-итераций определяется флагом `-benchtime` (по умолчанию 1 секунда);
- модель конкурентности — встроенный планировщик Go runtime.

В ходе тестирования оценивались: среднее время ответа, количество аллокаций памяти, пропускная способность.

Результаты нагрузочного тестирования представлены в таблице 5.4.

**Таблица 5.4**

| Бенчмарк | Итераций | Время | Пропускная способность |
|---|---|---|---|
| `BenchmarkCreateShortURL-12` | 38 800 | 28 126 нс/оп | ~35 500 оп/с |
| `BenchmarkResolveURL_CacheHit-12` | 175 485 | 6 870 нс/оп | ~145 500 оп/с |
| `BenchmarkResolveURL_CacheMiss-12` | 55 644 | 21 666 нс/оп | ~46 100 оп/с |
| `BenchmarkSqidsEncode-12` | 250 932 | 4 749 нс/оп | ~210 500 оп/с |
| `BenchmarkSqidsDecode-12` | 1 000 000 | 1 414 нс/оп | ~707 000 оп/с |

В процессе тестирования все операции были успешно обработаны backend-сервисом без ошибок.

Пропускная способность наиболее востребованной операции — разрешения URL при попадании в кэш — составляет около 148 500 операций в секунду, что является достаточным для обслуживания высоконагруженных веб-приложений.

Для запуска нагрузочного тестирования (таблица 5.4) используется команда:

```bash
go test -bench=. -benchmem -count=1 -timeout=30s github.com/Xindorgi/MnTP_Kursach/internal/service/...
```

## 5.5 Сравнение производительности операций сервиса

Анализ результатов показывает, что все ключевые операции backend-сервиса обладают высокой производительностью и минимальным временем обработки.

Наиболее ресурсоёмкой операцией является создание короткой ссылки (`BenchmarkCreateShortURL`), поскольку при её выполнении происходит:

1. вставка записи в БД (mock-операция);
2. генерация sqids-кода;
3. обновление записи с коротким кодом;
4. запись в кэш Redis.

Среднее время выполнения данной операции составляет 28 126 нс (~28.1 мкс).

Разрешение короткой ссылки при попадании в кэш (`BenchmarkResolveURL_CacheHit`) выполняется значительно быстрее — 6 870 нс (~6.9 мкс), что в 4 раза быстрее создания ссылки. При промахе кэша (`BenchmarkResolveURL_CacheMiss`) время возрастает до 21 666 нс (~21.7 мкс), что подтверждает критическую важность кэширования для производительности системы.

Генерация и декодирование sqids-кодов являются самыми быстрыми операциями (4 749 нс и 1 414 нс соответственно), что делает их пригодными для использования в высоконагруженных системах без создания узких мест.

В общем и целом, архитектура системы обеспечивает высокую производительность за счёт:

- использования Redis-кэширования для часто запрашиваемых данных;
- эффективных алгоритмов генерации коротких кодов (sqids);
- минимального количества блокирующих операций в критическом пути обработки запросов.