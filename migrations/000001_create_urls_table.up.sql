CREATE TABLE IF NOT EXISTS urls (
    id               BIGSERIAL       PRIMARY KEY,
    long_url         TEXT            NOT NULL,
    short_code       VARCHAR(10)     NOT NULL UNIQUE,
    management_token UUID            NOT NULL DEFAULT gen_random_uuid(),
    created_at       TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_urls_short_code ON urls(short_code);