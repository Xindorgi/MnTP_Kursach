# 4. Реализация

## 4.1 Реализация создания короткой ссылки

При создании короткой ссылки пользователь отправляет POST-запрос с длинным URL. Backend-сервис получает этот URL, проверяет его корректность и сохраняет в базе данных. После сохранения сервис получает числовой идентификатор записи и кодирует его в короткую строку с помощью библиотеки Sqids — это гарантирует, что каждый короткий код уникален и не требует проверки на коллизии, в отличие от хеш-функций. Затем короткий код сохраняется в записи, а результат помещается в кэш Redis для быстрого последующего доступа. Пользователю возвращается JSON с короткой ссылкой и management token для управления аналитикой.

Реализация создания короткой ссылки см. Листинг 1.

**Листинг 1. Создание короткой ссылки (хендлер → сервис → репозиторий)**

```go
// Хендлер: приём запроса, валидация, вызов сервиса
func (h *ShortenHandler) Handle(c *fiber.Ctx) error {
    var req shortenRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "invalid request body",
        })
    }
    // Проверка, что URL имеет схему http или https
    parsedURL, err := url.ParseRequestURI(req.URL)
    if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "invalid URL format. Must be a valid http or https URL",
        })
    }
    created, err := h.urlService.CreateShortURL(c.Context(), req.URL)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "failed to create short URL",
        })
    }
    return c.Status(fiber.StatusCreated).JSON(shortenResponse{
        ShortURL:        h.urlService.BaseURL() + "/" + *created.ShortCode,
        ShortCode:       *created.ShortCode,
        ManagementToken: created.ManagementToken,
    })
}

// Сервис: вставка в БД → кодирование ID → обновление → кэширование
func (s *URLService) CreateShortURL(ctx context.Context, longURL string) (*domain.URL, error) {
    url, err := s.urlRepo.Insert(ctx, longURL)          // 1. INSERT в PostgreSQL
    shortCode, err := s.sqids.Encode([]uint64{uint64(url.ID)}) // 2. Sqids.Encode(ID)
    s.urlRepo.UpdateShortCode(ctx, url.ID, shortCode)   // 3. UPDATE short_code
    url.ShortCode = &shortCode
    s.cacheRepo.Set(ctx, shortCode, longURL)             // 4. Кэш (best-effort)
    return url, nil
}

// Репозиторий: параметризованный INSERT с RETURNING
func (r *URLRepository) Insert(ctx context.Context, longURL string) (*domain.URL, error) {
    url := &domain.URL{}
    err := r.pool.QueryRow(ctx,
        `INSERT INTO urls (long_url) VALUES ($1)
         RETURNING id, long_url, short_code, management_token::text, created_at, updated_at`,
        longURL,
    ).Scan(&url.ID, &url.LongURL, &url.ShortCode, &url.ManagementToken, &url.CreatedAt, &url.UpdatedAt)
    return url, err
}
```

*Команда для проверки: `curl -X POST http://localhost:8080/api/v1/shorten -H "Content-Type: application/json" -d '{"url":"https://example.com"}'`*

## 4.2 Реализация редиректа и асинхронной записи аналитики

Когда пользователь переходит по короткой ссылке, сервер выполняет редирект на исходный длинный URL. Поскольку редирект — самый частый сценарий использования, он не должен задерживаться записью аналитики в базу данных. Поэтому аналитика собирается асинхронно: хендлер отправляет событие о переходе в канал и сразу возвращает ответ клиенту. Отдельная фоновая горутина (AnalyticsWorker) читает события из канала, обогащает их географическими данными через GeoIP и накапливает в батчи по 50 записей. Когда батч заполняется или проходит 1 секунда, данные массово вставляются в PostgreSQL одной транзакцией.

Важная техническая деталь: Fiber построен на FastHTTP, который использует zero-copy string interning — строки из HTTP-заголовков указывают на внутренний буфер, который переиспользуется при обработке следующего запроса. Поэтому перед отправкой в асинхронный канал строки необходимо клонировать через `strings.Clone()`, иначе к моменту обработки события воркером данные могут быть повреждены.

Реализация редиректа и асинхронной аналитики см. Листинг 2.

**Листинг 2. Редирект и асинхронная аналитика**

```go
// Хендлер редиректа: клонирование строк из FastHTTP, отправка события
func (h *RedirectHandler) Handle(c *fiber.Ctx) error {
    shortCode := c.Params("code")
    url, err := h.urlService.ResolveURL(c.Context(), shortCode)
    if err != nil {
        return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "URL not found"})
    }
    // FastHTTP zero-copy: клонируем перед отправкой в канал
    userAgent := strings.Clone(c.Get("User-Agent"))
    referer := strings.Clone(c.Get("Referer"))
    clientIP := strings.Clone(clientip.FromRequest(c))

    h.urlService.RecordClick(c.Context(), shortCode, clientIP, userAgent, referer)
    return c.Redirect(url.LongURL, fiber.StatusMovedPermanently)
}

// Сервис: неблокирующая отправка в канал (select с default — дроп при переполнении)
func (s *URLService) RecordClickByID(ctx context.Context, urlID int64, ip, userAgent, referer string) {
    event := domain.ClickEvent{
        URLID: urlID, IPAddress: ip,
        UserAgent: userAgent, Referer: referer,
        ClickedAt: time.Now(),
    }
    select {
    case s.eventsChan <- event:
    default: // канал полон — дропаем, не блокируя редирект
    }
}

// AnalyticsWorker: фоновая горутина, батчевая обработка
func (w *AnalyticsWorker) Start(ctx context.Context) {
    batch := make([]domain.ClickEvent, 0, BatchSize) // BatchSize = 50
    ticker := time.NewTicker(FlushInterval)           // FlushInterval = 1 сек
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            if len(batch) > 0 { w.flush(batch) }     // сброс при graceful shutdown
            return
        case event := <-w.events:
            if w.geoIP != nil { w.enrichWithGeoIP(&event) }
            batch = append(batch, event)
            if len(batch) >= BatchSize { w.flush(batch); batch = make(...) }
        case <-ticker.C:
            if len(batch) > 0 { w.flush(batch); batch = make(...) }
        }
    }
}

// GeoIP-обогащение: определение страны и города по IP
func (w *AnalyticsWorker) enrichWithGeoIP(event *domain.ClickEvent) {
    ip := net.ParseIP(event.IPAddress)
    if isPrivateIP(ip) { event.Country = "LOCAL"; return } // localhost/Docker
    record, err := w.geoIP.City(ip)
    if record.Country.IsoCode != "" { event.Country = record.Country.IsoCode }
    if name, ok := record.City.Names["en"]; ok { event.City = name }
}

// Batch-вставка: все события одной транзакцией через pgx.Batch
func (r *ClickRepository) BatchInsert(ctx context.Context, events []domain.ClickEvent) error {
    batch := &pgx.Batch{}
    for _, event := range events {
        batch.Queue(`INSERT INTO url_clicks (...) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
            event.URLID, event.IPAddress, event.UserAgent, event.Referer,
            event.Country, event.City, event.ClickedAt)
    }
    br := r.pool.SendBatch(ctx, batch)
    defer br.Close()
    for range events { if _, err := br.Exec(); err != nil { return err } }
    return nil
}
```

*Команда для проверки: `curl -v http://localhost:8080/abc123` (редирект) и `go test -count=1 -v -run TestE2EReferrerTracking ./internal/test_e2e/...` (E2E-тест рефереров)*

## 4.3 Реализация API для получения аналитики

Для получения статистики по короткой ссылке пользователь отправляет GET-запрос с указанием короткого кода и management token. Сервер проверяет токен — если он не совпадает с сохранённым при создании, доступ запрещается. После проверки сервер выполняет несколько агрегирующих SQL-запросов: подсчёт общего числа переходов, ежедневная статистика за последние 30 дней, топ-10 стран и топ-10 источников переходов (рефереров). Пустые значения стран нормализуются в "Unknown", пустые рефереры — в "Direct".

Реализация API аналитики см. Листинг 3.

**Листинг 3. Получение аналитики (хендлер → сервис → SQL-агрегация)**

```go
// Хендлер: проверка параметров, вызов сервиса
func (h *AnalyticsHandler) Handle(c *fiber.Ctx) error {
    shortCode := c.Params("code")
    token := c.Query("token")
    if token == "" {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
            "error": "management token is required",
        })
    }
    stats, err := h.urlService.GetAnalytics(c.Context(), shortCode, token)
    if err != nil {
        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
    }
    return c.JSON(stats)
}

// Сервис: проверка токена, получение статистики
func (s *URLService) GetAnalytics(ctx context.Context, shortCode, token string) (*domain.ClickStats, error) {
    url, err := s.urlRepo.FindByShortCode(ctx, shortCode)
    if url.ManagementToken != token {
        return nil, fmt.Errorf("invalid management token")
    }
    return s.clickRepo.GetStats(ctx, url.ID)
}

// Репозиторий: три агрегирующих SQL-запроса
func (r *ClickRepository) GetStats(ctx context.Context, urlID int64) (*domain.ClickStats, error) {
    stats := &domain.ClickStats{}
    // 1. Общее количество переходов
    r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM url_clicks WHERE url_id = $1`, urlID).Scan(&stats.TotalClicks)
    // 2. Ежедневные переходы за 30 дней
    rows, _ := r.pool.Query(ctx, `SELECT DATE(clicked_at)::text, COUNT(*)
        FROM url_clicks WHERE url_id = $1 AND clicked_at >= NOW() - INTERVAL '30 days'
        GROUP BY DATE(clicked_at) ORDER BY date DESC`, urlID)
    for rows.Next() { /* scan into stats.DailyClicks */ }
    // 3. Топ-10 стран (пустые → 'Unknown')
    rows, _ = r.pool.Query(ctx, `SELECT COALESCE(NULLIF(TRIM(country), ''), 'Unknown'), COUNT(*)
        FROM url_clicks WHERE url_id = $1
        GROUP BY COALESCE(NULLIF(TRIM(country), ''), 'Unknown')
        ORDER BY count DESC LIMIT 10`, urlID)
    for rows.Next() { /* scan into stats.TopCountries */ }
    // 4. Топ-10 рефереров (пустые → 'Direct')
    rows, _ = r.pool.Query(ctx, `SELECT COALESCE(NULLIF(TRIM(referer), ''), 'Direct'), COUNT(*)
        FROM url_clicks WHERE url_id = $1
        GROUP BY COALESCE(NULLIF(TRIM(referer), ''), 'Direct')
        ORDER BY count DESC LIMIT 10`, urlID)
    for rows.Next() { /* scan into stats.TopReferrers */ }
    return stats, nil
}
```

*Команда для проверки: `curl "http://localhost:8080/api/v1/analytics/abc123?token=<management_token>"`*

## 4.4 Реализация дашборда аналитики

Для визуального просмотра статистики реализована HTML-страница дашборда. Шаблон встраивается в бинарник приложения через директиву `//go:embed`, что позволяет разворачивать сервис как единый исполняемый файл без внешних статических ресурсов. Дашборд представляет собой одностраничное приложение на чистом JavaScript, которое через Fetch API обращается к эндпоинту аналитики и отображает данные в таблицах.

Реализация дашборда см. Листинг 4.

**Листинг 4. Дашборд с embed-шаблоном**

```go
//go:embed templates/dashboard.html
var dashboardHTML embed.FS

type DashboardHandler struct { tmpl *template.Template }

func NewDashboardHandler() *DashboardHandler {
    tmpl, _ := template.New("dashboard.html").ParseFS(dashboardHTML, "templates/dashboard.html")
    return &DashboardHandler{tmpl: tmpl}
}

func (h *DashboardHandler) Handle(c *fiber.Ctx) error {
    c.Set("Content-Type", "text/html; charset=utf-8")
    return h.tmpl.ExecuteTemplate(c, "dashboard.html", nil)
}
```

*Команда для проверки: открыть `http://localhost:8080/dashboard` в браузере*

## 4.5 Точка входа и Graceful Shutdown

Запуск приложения начинается с загрузки конфигурации из переменных окружения. Затем последовательно инициализируются репозитории: сначала PostgreSQL, а если он недоступен — in-memory fallback для разработки. Аналогично для Redis: при недоступности используется in-memory кэш. После инициализации всех зависимостей запускается AnalyticsWorker в фоновой горутине и HTTP-сервер. При получении сигнала SIGINT или SIGTERM сервер сначала останавливает приём новых запросов, затем AnalyticsWorker сбрасывает оставшиеся события из канала в базу данных и закрывает GeoIP-ридер.

Реализация точки входа см. Листинг 5.

**Листинг 5. Точка входа с graceful shutdown и in-memory fallback**

```go
func main() {
    cfg := config.Load()
    // Репозитории с fallback: PostgreSQL → in-memory
    pgRepo, err := postgres.NewURLRepository(cfg.PostgresConnString())
    if err != nil {
        urlRepo = postgres.NewInMemoryURLRepository()
        clickRepo = postgres.NewInMemoryClickRepository()
    } else {
        urlRepo = pgRepo
        clickRepo = postgres.NewClickRepositoryFromPool(pgRepo.Pool())
        migrator.RunUp(context.Background(), pgRepo.Pool(), "migrations")
    }
    // Кэш с fallback: Redis → in-memory
    redisCache, err := redis.NewCacheRepository(cfg.RedisAddr(), cfg.RedisPassword, 0)
    if err != nil {
        cacheRepo = redis.NewInMemoryCacheRepository()
    } else {
        cacheRepo = redisCache
    }
    // DI: worker → service → handlers → router
    analyticsWorker, _ := worker.NewAnalyticsWorker(clickRepo, cfg.GeoIPDBPath)
    urlSvc, _ := service.NewURLService(urlRepo, clickRepo, cacheRepo, analyticsWorker.EventsChan(), cfg.BaseURL)
    app := transport.SetupRoutes(/* handlers */)

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    go analyticsWorker.Start(ctx)  // фоновая обработка аналитики
    go app.Listen(cfg.AppAddr())   // HTTP-сервер

    <-ctx.Done()                   // ожидание Ctrl+C или SIGTERM
    app.Shutdown()                 // остановка сервера
    analyticsWorker.Close()        // закрытие GeoIP-базы
}
```

*Команда для запуска: `docker compose up --build` или `go run ./cmd/server/...`*