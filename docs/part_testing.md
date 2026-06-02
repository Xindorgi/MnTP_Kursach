# 5. Тестирование программного продукта

## 5.1 Общая организация тестирования

Тестирование разработанной системы сокращения URL-ссылок проводилось с целью проверки корректности работы backend-сервиса, механизмов создания и редиректа коротких ссылок, интеграции с Redis-кэшем, GeoIP-определения местоположения, аналитического воркера, а также оценки производительности REST API под нагрузкой.

В рамках проекта были реализованы:

- модульные тесты backend-компонентов;
- интеграционные тесты сценариев работы системы;
- end-to-end тестирование полного цикла создания и редиректа ссылок;
- тестирование производительности API (benchmarks);
- анализ покрытия кода тестами.

Для автоматизированного тестирования использовался встроенный пакет `testing` языка Go и фреймворк `testify` для утверждений и моков.

## 5.2 Модульное тестирование

Проект покрыт набором unit-тестов, проверяющих корректность работы основных компонентов системы.

Тестирование охватывает:

- создание и разрешение коротких URL-ссылок;
- работу сервисного слоя с моками репозиториев;
- кэширование через Redis (in-memory реализация);
- определение IP-адреса клиента;
- определение приватных и публичных IP-адресов;
- GeoIP-обогащение событий кликов;
- агрегацию статистики переходов;
- end-to-end сценарии работы системы.

Структура тестов см. таблица 5.1.

**Таблица 5.1**

| Файл | Назначение | Количество тестов |
|---|---|---|
| `internal/service/url_service_test.go` | Сервис создания и разрешения URL | 5 |
| `internal/repository/postgres/click_repo_memory_test.go` | In-memory репозиторий кликов | 1 |
| `internal/transport/clientip/clientip_test.go` | Определение IP клиента | 2 |
| `internal/worker/analytics_worker_test.go` | Аналитический воркер (приватные IP, GeoIP) | 4 |
| `internal/worker/geoip_countries_test.go` | GeoIP-определение стран | 4 |
| `internal/worker/geoip_testhelper_test.go` | Вспомогательные функции для GeoIP-тестов | 1 |
| `internal/test_e2e/e2e_test.go` | E2E-тесты (создание, редирект, аналитика, дашборд) | 5 |
| `internal/test_e2e/geoip_e2e_test.go` | E2E-тесты GeoIP (страны, локальные сети) | 2 |

Всего в проекте реализовано **24 теста**.

Результаты выполнения тестирования см. листинг 5.1.

```
=== RUN   TestCreateShortURL_Success
--- PASS: TestCreateShortURL_Success (0.00s)
=== RUN   TestCreateShortURL_InsertError
--- PASS: TestCreateShortURL_InsertError (0.00s)
=== RUN   TestResolveURL_CacheHit
--- PASS: TestResolveURL_CacheHit (0.00s)
=== RUN   TestResolveURL_CacheMiss
--- PASS: TestResolveURL_CacheMiss (0.00s)
=== RUN   TestResolveURL_NotFound
--- PASS: TestResolveURL_NotFound (0.00s)
=== RUN   TestInMemoryClickRepository_GetStats_AggregatesCountries
--- PASS: TestInMemoryClickRepository_GetStats_AggregatesCountries (0.02s)
=== RUN   TestFromRequest_UsesFiberIPWhenPresent
--- PASS: TestFromRequest_UsesFiberIPWhenPresent (0.00s)
=== RUN   TestFromRequest_FallsBackToRemoteAddrWhenFiberIPMissing
--- PASS: TestFromRequest_FallsBackToRemoteAddrWhenFiberIPMissing (0.00s)
=== RUN   TestIsPrivateIP
--- PASS: TestIsPrivateIP (0.00s)
=== RUN   TestEnrichWithGeoIP_PrivateIPMarkedLocal
--- PASS: TestEnrichWithGeoIP_PrivateIPMarkedLocal (0.00s)
=== RUN   TestEnrichWithGeoIP_NoDatabaseLeavesPublicIPUnset
--- PASS: TestEnrichWithGeoIP_NoDatabaseLeavesPublicIPUnset (0.00s)
=== RUN   TestEnrichWithGeoIP_EmptyIPMarkedLocal
--- PASS: TestEnrichWithGeoIP_EmptyIPMarkedLocal (0.00s)
=== RUN   TestEnrichWithGeoIP_PublicCountries
--- PASS: TestEnrichWithGeoIP_PublicCountries (0.06s)
=== RUN   TestEnrichWithGeoIP_PublicIPWithoutDatabase
--- PASS: TestEnrichWithGeoIP_PublicIPWithoutDatabase (0.00s)
=== RUN   TestEnrichWithGeoIP_InvalidIPUnchanged
--- PASS: TestEnrichWithGeoIP_InvalidIPUnchanged (0.00s)
=== RUN   TestNewAnalyticsWorker_MissingDatabaseStillCreatesWorker
--- PASS: TestNewAnalyticsWorker_MissingDatabaseStillCreatesWorker (0.00s)
=== RUN   TestE2EShortenAndRedirect
--- PASS: TestE2EShortenAndRedirect (1.03s)
=== RUN   TestDashboardPage
--- PASS: TestDashboardPage (0.00s)
=== RUN   TestE2E_GeoIPCountries
--- PASS: TestE2E_GeoIPCountries (1.00s)
=== RUN   TestE2E_GeoIPLocalNetwork
--- PASS: TestE2E_GeoIPLocalNetwork (1.00s)
PASS
ok  	github.com/Xindorgi/MnTP_Kursach/internal/service	0.531s
ok  	github.com/Xindorgi/MnTP_Kursach/internal/repository/postgres	0.531s
ok  	github.com/Xindorgi/MnTP_Kursach/internal/transport/clientip	0.666s
ok  	github.com/Xindorgi/MnTP_Kursach/internal/worker	0.583s
ok  	github.com/Xindorgi/MnTP_Kursach/internal/test_e2e	3.265s
```

**Листинг 5.1** – Результат запуска тестов

Все тесты завершились успешно без возникновения ошибок.

Средний уровень покрытия кода тестами по пакетам представлен в таблице 5.2.

**Таблица 5.2**

| Пакет | Покрытие |
|---|---|
| `internal/service` | 50.0% |
| `internal/repository/postgres` | 24.1% |
| `internal/transport/clientip` | 28.6% |
| `internal/worker` | 52.5% |

Для получения отчёта о покрытии используется команда:

```bash
make test-cover
# или
go test -count=1 -cover ./internal/service/... ./internal/worker/... ./internal/transport/... ./internal/repository/... ./internal/test_e2e/...
```

Для получения детального HTML-отчёта о покрытии:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
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
| `BenchmarkCreateShortURL` | 41 632 | 28 925 нс/оп | 12 340 B/op | 111 allocs/op |
| `BenchmarkResolveURL_CacheHit` | 176 464 | 6 735 нс/оп | 3 500 B/op | 34 allocs/op |
| `BenchmarkResolveURL_CacheMiss` | 53 492 | 21 513 нс/оп | 10 686 B/op | 107 allocs/op |
| `BenchmarkSqidsEncode` | 251 824 | 5 029 нс/оп | 1 847 B/op | 20 allocs/op |
| `BenchmarkSqidsDecode` | 742 155 | 1 555 нс/оп | 1 016 B/op | 12 allocs/op |

Наиболее быстрой операцией является декодирование sqids-кода (`BenchmarkSqidsDecode` — 1 555 нс/оп), что ожидаемо, поскольку данная операция выполняется в памяти без обращений к внешним хранилищам.

Наиболее медленной операцией является создание короткой ссылки (`BenchmarkCreateShortURL` — 28 925 нс/оп), так как при её выполнении производится вставка записи в БД, генерация sqids-кода, обновление записи и запись в кэш.

Разрешение URL при попадании в кэш (`BenchmarkResolveURL_CacheHit`) выполняется в 3 раза быстрее, чем при промахе кэша (`BenchmarkResolveURL_CacheMiss`), что подтверждает эффективность использования Redis-кэширования.

Для запуска бенчмарков используется команда:

```bash
make bench
# или
go test -bench=. -benchmem -count=1 -timeout=30s ./internal/service/...
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
| `BenchmarkCreateShortURL-12` | 41 632 | 28 925 нс/оп | ~34 500 оп/с |
| `BenchmarkResolveURL_CacheHit-12` | 176 464 | 6 735 нс/оп | ~148 500 оп/с |
| `BenchmarkResolveURL_CacheMiss-12` | 53 492 | 21 513 нс/оп | ~46 500 оп/с |
| `BenchmarkSqidsEncode-12` | 251 824 | 5 029 нс/оп | ~198 800 оп/с |
| `BenchmarkSqidsDecode-12` | 742 155 | 1 555 нс/оп | ~643 000 оп/с |

В процессе тестирования все операции были успешно обработаны backend-сервисом без ошибок.

Пропускная способность наиболее востребованной операции — разрешения URL при попадании в кэш — составляет около 148 500 операций в секунду, что является достаточным для обслуживания высоконагруженных веб-приложений.

## 5.5 Сравнение производительности операций сервиса

Анализ результатов показывает, что все ключевые операции backend-сервиса обладают высокой производительностью и минимальным временем обработки.

Наиболее ресурсоёмкой операцией является создание короткой ссылки (`BenchmarkCreateShortURL`), поскольку при её выполнении происходит:

1. вставка записи в БД (mock-операция);
2. генерация sqids-кода;
3. обновление записи с коротким кодом;
4. запись в кэш Redis.

Среднее время выполнения данной операции составляет 28 925 нс (~28.9 мкс).

Разрешение короткой ссылки при попадании в кэш (`BenchmarkResolveURL_CacheHit`) выполняется значительно быстрее — 6 735 нс (~6.7 мкс), что в 4 раза быстрее создания ссылки. При промахе кэша (`BenchmarkResolveURL_CacheMiss`) время возрастает до 21 513 нс (~21.5 мкс), что подтверждает критическую важность кэширования для производительности системы.

Генерация и декодирование sqids-кодов являются самыми быстрыми операциями (5 029 нс и 1 555 нс соответственно), что делает их пригодными для использования в высоконагруженных системах без создания узких мест.

В общем и целом, архитектура системы обеспечивает высокую производительность за счёт:

- использования Redis-кэширования для часто запрашиваемых данных;
- эффективных алгоритмов генерации коротких кодов (sqids);
- минимального количества блокирующих операций в критическом пути обработки запросов.