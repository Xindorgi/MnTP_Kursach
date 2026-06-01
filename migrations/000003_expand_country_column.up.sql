-- Allow non-ISO values such as LOCAL for private/loopback IPs.
ALTER TABLE url_clicks ALTER COLUMN country TYPE VARCHAR(16);
