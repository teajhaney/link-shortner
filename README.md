# Link Shortener

A small Go URL shortener backed by PostgreSQL.

## Requirements

- Go 1.26 or newer
- PostgreSQL
- Air, optional, for live reload during development

## Configuration

Create a `.env` file in the project root:

```env
DATABASE_URL=postgres://username:password@host:5432/database?sslmode=require
JWT_SECRET=replace-with-a-random-32-byte-secret
PUBLIC_BASE_URL=http://localhost:8080
JWT_ISSUER=link-shortner
```

The application loads `.env` automatically. Do not commit this file because it contains database credentials and signing secret material.

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

Every route is JSON. Protected routes take the access token as a bearer token
in the `Authorization` header; the redirect is the only public route.

Errors always come back in the same shape:

```json
{ "error": "missing or malformed Authorization header" }
```

### Auth

#### Sign in

Public. Returns the access token used by every protected route, plus the
refresh token that renews it.

```bash
curl -X POST http://localhost:8080/api/auth/signin \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"password123"}'
```

```json
{
  "code": 200,
  "message": "Signin successful",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 900
}
```

Bad credentials return `401` with `{"error":"invalid email or password"}`.

#### Refresh the access token

Public. The refresh token travels in the body, not a header. It is rotated on
every use, so the token in the response replaces the one you sent.

```bash
curl -X POST http://localhost:8080/api/auth/refresh \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"<refresh_token>"}'
```

```json
{
  "code": 200,
  "message": "Token refreshed",
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "Bearer",
  "expires_in": 900
}
```

An expired, revoked, replayed, or unknown refresh token returns `401`; the fix
is to sign in again.

#### Verify a token

Public. Confirms a token is still valid and reads its user ID without touching
a protected route. It checks the revocation list, so a logged-out token reports
as invalid.

```bash
curl http://localhost:8080/api/auth/verify \
  -H 'Authorization: Bearer <access_token>'
```

```json
{
  "code": 200,
  "message": "Token is valid",
  "user_id": "a1b2c3d4-..."
}
```

#### Sign out

Public. Pass the access token as a bearer token, a refresh token in the body,
or both; both are revoked. It is forgiving, so signing out twice still returns
`200`.

```bash
curl -X POST http://localhost:8080/api/auth/logout \
  -H 'Authorization: Bearer <access_token>' \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"<refresh_token>"}'
```

```json
{
  "code": 200,
  "message": "Signed out"
}
```

Presenting neither credential returns `400`.

### Users

#### Create an account

Public. This is the only users route that does not need a token.

```bash
curl -X POST http://localhost:8080/api/user/create \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ada Lovelace","email":"you@example.com","password":"password123"}'
```

```json
{
  "code": 201,
  "message": "User created successfully"
}
```

#### Get your user by email

Protected and self-only: the authenticated caller may look up their own record
and nobody else's. Asking for another user's email returns `403`.

```bash
curl 'http://localhost:8080/api/user?email=you@example.com' \
  -H 'Authorization: Bearer <access_token>'
```

```json
{
  "code": 200,
  "message": "User retrieved successfully",
  "data": {
    "id": "a1b2c3d4-...",
    "name": "Ada Lovelace",
    "email": "you@example.com",
    "created_at": "2026-09-26T12:00:00Z",
    "updated_at": "2026-09-26T12:00:00Z"
  }
}
```

#### Get your user by ID

Protected and self-only. The ID in the path must match the token's user ID.

```bash
curl http://localhost:8080/api/user/a1b2c3d4-... \
  -H 'Authorization: Bearer <access_token>'
```

The response shape is the same as the email lookup above.

#### Update your user

Protected and self-only. All three fields are optional; send only what you
want to change. An empty body returns `400`.

```bash
curl -X PATCH http://localhost:8080/api/user/a1b2c3d4-... \
  -H 'Authorization: Bearer <access_token>' \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ada King"}'
```

```json
{
  "code": 200,
  "message": "User updated successfully",
  "data": {
    "id": "a1b2c3d4-...",
    "name": "Ada King",
    "email": "you@example.com",
    "created_at": "2026-09-26T12:00:00Z",
    "updated_at": "2026-09-26T12:30:00Z"
  }
}
```

#### Delete your user

Protected and self-only. Deleting the account also deletes its short links,
because `urls.user_id` cascades on delete.

```bash
curl -X DELETE http://localhost:8080/api/user/a1b2c3d4-... \
  -H 'Authorization: Bearer <access_token>'
```

```json
{
  "code": 200,
  "message": "User deleted successfully"
}
```

#### List all users

Not exposed. Without an admin role, returning every account's name and email
is an information leak, so the route is commented out in `internal/users/routes.go`.

### Links

#### Create a short link

Protected. The link is saved under the authenticated user's account, so it
shows up in their list and stats. The owner comes from the token, never from
the request body.

```bash
curl -X POST http://localhost:8080/api/shorten \
  -H 'Authorization: Bearer <access_token>' \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com"}'
```

```json
{
  "short_url": "http://localhost:8080/kP7x2QaM",
  "code": "kP7x2QaM",
  "long_url": "https://example.com"
}
```

#### List your links

Protected. Returns every link the caller has shortened, newest first. Links
created by other users are never included.

```bash
curl http://localhost:8080/api/links \
  -H 'Authorization: Bearer <access_token>'
```

```json
{
  "code": 200,
  "message": "Links retrieved successfully",
  "data": [
    {
      "code": "kP7x2QaM",
      "short_url": "http://localhost:8080/kP7x2QaM",
      "long_url": "https://example.com",
      "clicks": 3,
      "created_at": "2026-09-26T12:00:00Z"
    }
  ]
}
```

#### Get statistics

Protected. Only the link's owner sees its stats, and reading them does not
count as a click. A code that belongs to somebody else answers `404`, the same
as a code that was never issued, so the caller learns nothing about codes they
do not own.

```bash
curl http://localhost:8080/api/stats/kP7x2QaM \
  -H 'Authorization: Bearer <access_token>'
```

```json
{
  "code": "kP7x2QaM",
  "short_url": "http://localhost:8080/kP7x2QaM",
  "long_url": "https://example.com",
  "clicks": 3,
  "created_at": "2026-09-26T12:00:00Z"
}
```

#### Redirect

Public and owner-blind: the whole point of a short link is that anybody holding
the code can follow it, with or without an account.

```bash
curl -i http://localhost:8080/kP7x2QaM
```

The API responds with `302 Found` and the original URL in the `Location`
header. Following the link records one click; an unknown code returns `404`.

Short links are stored in PostgreSQL, so they remain available after the API process is stopped and restarted.

## Development

Run all Go tests and compile all packages:

```bash
go test ./...
```
