# Link Shortener

A small Go URL shortener backed by PostgreSQL.

## Requirements

- Go 1.25 or newer
- PostgreSQL
- Air, optional, for live reload during development

## Configuration

Create a `.env` file in the project root:

```env
DATABASE_URL=postgres://username:password@host:5432/database?sslmode=require
```

The application loads `.env` automatically. Do not commit this file because it contains database credentials.

## Database Setup

Apply the initial schema:

```bash
psql "$DATABASE_URL" \
  -v ON_ERROR_STOP=1 \
  -f internal/migrations/0001_init.sql
```

For future schema changes, create the next numbered migration in `internal/migrations`, then apply it explicitly:

```bash
psql "$DATABASE_URL" \
  -v ON_ERROR_STOP=1 \
  -f internal/migrations/0002_add_expiration.sql
```

The application does not run migrations automatically.

## Run the API

Run directly:

```bash
go run ./cmd/api
```

Run with Air:

```bash
air
```

The server listens on `http://localhost:8080`.

## API

### Create a short link

```bash
curl -X POST http://localhost:8080/api/shorten \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com"}'
```

Example response:

```json
{
  "short_url": "http://localhost:8080/1",
  "code": "1",
  "long_url": "https://example.com"
}
```

### Redirect

Open the returned short URL, or request it directly:

```bash
curl -i http://localhost:8080/1
```

The API responds with a `302 Found` redirect to the original URL.

### Get statistics

```bash
curl http://localhost:8080/api/stats/1
```

Short links are stored in PostgreSQL, so they remain available after the API process is stopped and restarted.

## Development

Run all Go tests and compile all packages:

```bash
go test ./...
```
