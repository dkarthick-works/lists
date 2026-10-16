# Lists

A small PWA for nested lists. React + Vite frontend, Go (chi, sqlc, pgx) backend, PostgreSQL.

## Run

```bash
# backend — http://localhost:8082 (reads backend/.env, applies the schema on startup)
cd backend && air        # hot reload; or: go run ./cmd/server

# frontend — http://localhost:5175 (proxies /api to the backend)
cd frontend && npm install && npm run dev
```

`backend/.env` (copy the root `.env.example`) needs `DATABASE_URL`, `GOAUTH_BASE_URL` and
`JWT_SECRET` (the same value Goauth signs with).

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
- Every other `/api` route verifies the Bearer token itself, using Goauth's `JWT_SECRET`:
  it checks the HS256 signature and expiry in-process and reads the user id from the
  token, with no call to Goauth.

The refresh cookie is `Secure`. Chrome and Firefox accept it on `http://localhost`;
Safari does not, so in Safari a reload logs you out unless you serve over HTTPS.

## MCP

`/mcp` lets AI agents (Claude Code, Cursor, …) read and add to a user's lists over MCP
(Streamable HTTP, stateless). Tools: `search_lists`, `list_lists`, `get_list`,
`create_list` and `add_entry`. They call the same functions as the REST handlers, so long
text still becomes a page.

To connect, create a key under Profile → Developer settings and paste the command shown:

```bash
claude mcp add --transport http lists https://<host>/mcp --header "Authorization: Bearer lst_…"
```

Keys (`lst_…`) live in `lists.api_keys` as SHA-256 hashes; the raw key is shown once.
They work only on `/mcp`, never on `/api`, and stop working when revoked. The MCP SDK
rejects requests that reach `127.0.0.1` with a non-localhost `Host` (DNS-rebinding
protection), so a reverse proxy on the same host must forward to a non-loopback address
or keep the `Host` as localhost.

## Data model

One table, `lists.items`, forms a tree: a list is an item with `is_list = true`, an entry
is an item whose `parent_id` is a list, and an entry can itself be a list. Editing
anything bumps `updated_at` on every list above it, which drives "recently edited".
Entries are ordered by `position` within their list; new entries go to the end.
Completed entries (`completed_at`) sort after the open ones but keep their position.
Top-level lists can be pinned (`pinned_at`); pinned lists get their own section on the home
page and are left out of "recently edited".

## Pages (experimental)

Text longer than `PAGE_WORD_THRESHOLD` words becomes a page: an entry whose `text` is a
short generated title and whose `body` holds the full text. Typed into a list, it is a
page in that list; typed on the home page, it creates a list with that title containing
the page. Pages hold no entries. A new page is titled at once with its first
`PAGE_INITIAL_TITLE_WORDS` words; if `OPENROUTER_API_KEY` and `OPENROUTER_MODEL` are set,
a generated title replaces that in the background, unless the user has renamed it first.
All the knobs are in `.env.example`.

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

## Android app

`android/` is a thin Kotlin shell: one activity with a WebView that loads the web app.
The only native UI is the reminder screen, and there is no separate API client.

```bash
cd android
adb reverse tcp:5175 tcp:5175   # let the device reach the Vite dev server
./gradlew installDebug          # debug build loads http://localhost:5175
```

The release build loads `LISTS_URL` from `android/gradle.properties`; set it to the
deployed app's HTTPS address before running `./gradlew assembleRelease`.

## Reminders (Android only)

An entry can have one reminder: `remind_at`, a wall-clock time with no zone (08:00 stays
08:00 wherever the phone is), and an optional RFC 5545 `remind_rule` that repeats it. The
server only stores them (`PUT /api/items/{id}/reminder`, `GET /api/reminders`) and checks
the rule against what the picker writes: `FREQ` daily to yearly, `INTERVAL`, `BYDAY`,
`BYMONTHDAY`, and `COUNT` or `UNTIL`.

The web app offers reminders only inside the Android app, where `window.Android` exists:
a clock in the entry menu opens the picker (`/lists/:id/remind/:entryId`). After every
change, and whenever the app comes back to the front, `frontend/src/android.ts` hands the
phone the full reminder list. The phone sets one exact alarm for the soonest one and,
when it rings, shows `ReminderActivity` over any app or the lock screen. Done there takes
a double-tap and reaches the server on the next sync; snooze stays on the phone. A
completed entry's reminder is paused until it is reopened. Edits made in a browser or
over MCP reach the phone the next time the app is opened.

Grant these once for the app (it opens each screen the first time a reminder exists):
notifications, **Alarms & reminders** (exact alarms), **Full-screen notifications**,
and **Display over other apps** (lets the reminder open over an unlocked phone instead
of only showing a banner). Turn off battery optimisation for it too. Under Do Not
Disturb the reminder still opens but stays silent. Force-stopping the app cancels its
alarm until it is opened again.

Checks for the rule logic: `node frontend/reminder.check.ts` and, in `android/`,
`./gradlew testDebugUnitTest`.
