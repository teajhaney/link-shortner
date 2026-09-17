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

Migrations run automatically when the API starts. The SQL files in
`internal/migrations` are embedded into the binary, and on startup the API
applies any file that is not already recorded in the `schema_migrations` table.
Each file runs in its own transaction, so a failed migration leaves the schema
unchanged and nothing is recorded as applied.

To add a schema change, create the next numbered file in
`internal/migrations` and restart the API:

```
internal/migrations/0003_add_expiration.sql
```

No `psql` step is needed, and there is no separate migration command to run in
CI or on deploy.

Each file is idempotent (`CREATE TABLE IF NOT EXISTS`), so the runner is
safe against a database that was set up by hand before migrations were
automatic: it reconciles the schema and records both files as applied.

### Required privileges

The role in `DATABASE_URL` needs DDL privileges, not just DML, because
migrations run as the API starts. `0002_add_users.sql` calls
`CREATE EXTENSION pgcrypto`, so the role also needs to create that extension.
On Neon, grant the role `CREATE` on the schema it migrates.

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
  "short_url": "http://localhost:8080/kP7x2QaM",
  "code": "kP7x2QaM",
  "long_url": "https://example.com"
}
```

### Redirect

Open the returned short URL, or request it directly:

```bash
curl -i http://localhost:8080/kP7x2QaM
```

The API responds with a `302 Found` redirect to the original URL.

### Get statistics

```bash
curl http://localhost:8080/api/stats/kP7x2QaM
```

Short links are stored in PostgreSQL, so they remain available after the API process is stopped and restarted.

## Development

Run all Go tests and compile all packages:

```bash
go test ./...
```
