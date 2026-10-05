# Lists

A small PWA for nested lists. React + Vite frontend, Go (chi, sqlc, pgx) backend, PostgreSQL.

## Run

```bash
# backend — http://localhost:8082 (reads backend/.env, applies the schema on startup)
cd backend && air        # hot reload; or: go run ./cmd/server

# frontend — http://localhost:5175 (proxies /api to the backend)
cd frontend && npm install && npm run dev
```

`backend/.env` (see `.env.example`) needs `DATABASE_URL` and `GOAUTH_BASE_URL`, and
should have `JWT_SECRET` (the same value Goauth signs with).

## Database

Tables live in the `lists` schema. The app connects as the `lists_app` role, which owns
that schema and nothing else. Provisioning — the role, the schema and the `pg_trgm`
extension — is in `backend/db/migrations/000_schema.sql` and is run once by an admin.
The numbered migrations after it are applied by the server on every start.

After changing anything under `backend/db/`, run `sqlc generate` in `backend/`.

## Auth

Goauth handles accounts. The browser only talks to this backend:

- `/api/auth/*` is reverse-proxied to Goauth's `/auth/*`, with the refresh cookie's path
  rewritten to match. Same origin is what makes the `SameSite=Strict` cookie work.
- Every other `/api` route verifies the Bearer token itself: with Goauth's `JWT_SECRET`
  set, it checks the HS256 signature and expiry in-process and reads the user id from the
  token, with no call to Goauth. Without the secret it falls back to asking Goauth's
  `GET /auth/me` (cached for a minute), which works but adds a slow round trip.

The refresh cookie is `Secure`. Chrome and Firefox accept it on `http://localhost`;
Safari does not, so in Safari a reload logs you out unless you serve over HTTPS.

## Data model

One table, `lists.items`, forms a tree: a list is an item with `is_list = true`, an entry
is an item whose `parent_id` is a list, and an entry can itself be a list. Editing
anything bumps `updated_at` on every list above it, which drives "recently edited".
Entries are ordered by `position` within their list; new entries go to the end.
Completed entries (`completed_at`) sort after the open ones but keep their position.
Top-level lists can be pinned (`pinned_at`); pinned lists get their own section on the home
page and are left out of "recently edited".

## Search

`GET /api/lists/autocomplete?q=` returns up to 8 lists, at any depth, whose title contains
the query or is a near miss of it (pg_trgm word similarity, so small typos still match).
Prefix matches rank first. A partial GIN trigram index on list titles backs both halves.

## Production

One image serves both the API and the frontend: the Dockerfile builds the frontend, then
compiles the Go server with `-tags prod`, which embeds `frontend/dist` into the binary.

```bash
cp .env.example .env      # set DATABASE_URL to a host reachable from the container
docker compose up -d --build
```

The app listens on port 8082 in the container, published on host port 8085, and joins
the external `coolify` network. The database must already be provisioned (see Database).
