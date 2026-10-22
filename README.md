# Mystic Square — Levels Project

Mystic Square is a browser-based sliding-puzzle game built around the classic 15 puzzle. The project provides a REST API for level configurations, authentication, users, and game sessions, plus an embedded server-rendered web UI for playing games and administering levels and user roles.

API authentication uses bcrypt password hashes and expiring HS256 JWTs. Browser authentication uses a separate HttpOnly session cookie. Level mutations and user administration are restricted to admins. PostgreSQL persistence is provided through GORM mappings and golang-migrate migrations; isolated in-memory repositories are used when no database URL is configured.

The HTTP API exposes public level browsing and authentication, admin-only level management and user administration, and authenticated game sessions. The web UI exposes login, registration, logout, the authenticated game page, and admin pages at `/levels` and `/users`. New registrations always receive the `user` role; an admin account must be provisioned or promoted out-of-band by a trusted operator.

## Security design

### Roles

| Role | Permissions |
|---|---|
| `admin` | Browse and play levels; create, update, and delete levels through the API; use the web admin pages; list users; and change a user's role. |
| `user` | Browse and play levels and access only games owned by that account. |

Registration always creates the `user` role. Clients cannot choose roles, IDs, password hashes, or system timestamps. Admin accounts must be provisioned or promoted out-of-band by a trusted operator; there is no public admin-registration path.

### Access-control matrix

Public routes require no token:

- `GET /health`
- `GET /swagger/*any`
- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `GET /api/v1/levels`
- `GET /api/v1/levels/:id`

Authenticated users or admins may access:

- `POST /api/v1/games`
- `GET /api/v1/games` — users see their games; admins see all
- `GET /api/v1/games/:id` — owner or admin only
- `POST /api/v1/games/:id/move` and `/moves` — owner or admin only

Admins only may access the following routes. `AuthMiddleware` executes before `RequireRole`:

- `POST /api/v1/levels`
- `PUT /api/v1/levels/:id`
- `DELETE /api/v1/levels/:id`
- `GET /api/v1/users`
- `PUT /api/v1/users/:id/role`

`POST /api/v1/users` is not exposed; user creation is performed only through registration. No librarian or staff role exists in this project.

### DTO and credential rules

- `RegisterRequest` accepts only `email`, `full_name`, and `password`; strict JSON decoding rejects unknown fields such as `role`, `id`, and `password_hash`.
- `LoginRequest` accepts only `email` and `password`.
- The update-user-role request accepts only `role`, and only admins can reach the endpoint.
- Game creation accepts only `level_id`. The owner ID comes from the validated JWT, not client-controlled JSON.
- `model.User.PasswordHash` is tagged `json:"-"`; API responses omit it.
- Passwords use bcrypt hashes with `bcrypt.DefaultCost`. Login returns the same HTTP `401` response, `invalid credentials`, for unknown emails and wrong passwords.
- JWTs use HS256 with `user_id`, `role`, and `exp` claims and a 24-hour lifetime. The key is read only from `JWT_SECRET`; startup requires at least 32 bytes.
- Authentication and role failures use `AbortWithStatusJSON` and stop the handler chain.
- Request logging records method, path, status, and duration but never reads or logs request bodies, so password fields are not logged.

## 1. Project overview

Players solve a sliding puzzle by moving numbered tiles into the empty cell until the board is in its solved order. A level configures a puzzle's theme, objective, difficulty, board dimensions, and optional limits.

The product defines two roles:

| Role | Intended permissions |
|---|---|
| `admin` | Browse and play levels; create/update/delete levels through the API; use web admin pages; list users and update roles. |
| `user` | Browse and play levels; cannot create levels. |

The server enforces these permissions at the HTTP boundary. A `user` can play games but cannot create or modify levels. The authenticated browser UI is served at `/game` and uses the same game services with an HttpOnly session cookie. Administrators receive additional `/levels` and `/users` navigation links; regular users do not.

### Puzzle rules and player flow

- A board with `rows × columns` cells contains `rows × columns - 1` numbered tiles and one empty cell. A standard 15 puzzle uses a 4×4 board and tiles 1–15.
- A legal move slides a tile horizontally or vertically adjacent to the empty cell into that cell. Diagonal moves are not allowed.
- The solved arrangement orders the tiles from left to right and top to bottom, with the empty cell last.
- A game session should start with a solvable arrangement and track the board, move count, and elapsed time separately from the level definition.
- A player selects a level, starts a game, makes legal moves, and sees whether they solved the puzzle or reached an enabled time or move limit.

### Creating a level

An authenticated admin can create a level through either the API or the browser `/levels` page. The application validates the input, assigns system-managed identifiers and timestamps, and makes the level available to play. The browser page allows existing levels to edit only `title` and `description`; the API retains its full level replacement and deletion operations. Regular users may browse and play levels but cannot create, update, or delete them.

## 2. Level resource and fields

The `levels` variant is defined by `internal/app/variants_test.go`. Its four required string fields, optional integer, and optional text field are:

| Field | Type | Request requirement | Purpose |
|---|---|---|---|
| `title` | string | Required | Name shown in the level list and game. |
| `theme` | string | Required | Setting or theme, such as a forest or space station. |
| `objective` | string | Required | What the player must accomplish. |
| `difficulty` | string | Required | Difficulty label; this is also the list-filter field. The project defines `low`, `medium`, or `hard` as its difficulty values. The JSON API currently validates that the field is non-blank; the browser admin form accepts the three defined values. |
| `move_limit` | integer | Optional | Maximum permitted moves; omitted or `-1` uses the default maximum of `10000`. |
| `description` | string | Optional | Additional details or instructions. |

Required strings must be present and must not be blank. The optional fields may be omitted; in API responses they remain present as `null` when unset. When supplied, `move_limit` must be an integer, not a fractional number or another JSON type.

The puzzle domain also has these configuration and system-managed fields:

| Field | Type | Request behavior | Purpose |
|---|---|---|---|
| `columns` | integer | Optional; defaults to `4` | Number of board columns. Values from 2 through 100 are accepted. |
| `rows` | integer | Optional; defaults to `4` | Number of board rows. Values from 2 through 100 are accepted. |
| `time_limit` | integer | Optional; omitted means unlimited | Allowed play time in seconds; `-1` means unlimited. Values from `-1` through `3600` are accepted. |
| `id` | integer | Assigned by the service | Positive, monotonically increasing resource identifier. IDs are not reused after deletion. |
| `created_at` | timestamp | Assigned by the service | Creation time in RFC 3339 format. |
| `updated_at` | timestamp | Assigned by the service | Last update time, refreshed when the resource is updated. |

`move_limit` may be from `-1` through `10000`; `0` is a configured limit, while an omitted value or `-1` uses the default maximum of `10000` for game sessions. `time_limit` continues to use the unlimited convention. An omitted move limit is represented as `null` in the resource.

A level response has this shape (timestamps and ID are examples):

```json
{
  "id": 1,
  "title": "Forest Gate",
  "theme": "forest",
  "objective": "Find the hidden path",
  "difficulty": "medium",
  "move_limit": 100,
  "description": "Arrange the tiles to open the gate.",
  "columns": 4,
  "rows": 4,
  "time_limit": null,
  "created_at": "2026-10-05T12:00:00.123456Z",
  "updated_at": "2026-10-05T12:00:00.123456Z"
}
```

The board dimensions and `time_limit` are additional level settings; the variant's four required strings and two optional fields above define the required/optional variant contract. Runtime tile arrangement, elapsed time, and moves made belong to a game session, not to this level resource.

## 3. How to run and test

Use Go 1.27.1. Without `DATABASE_URL`, the application uses isolated in-memory repositories, which are useful for local development and HTTP tests. Set a strong, unique JWT signing secret (at least 32 bytes) before starting the application from the repository root:

```bash
export JWT_SECRET="replace-with-a-random-secret-of-at-least-32-bytes"
go run ./cmd/api
```

Startup fails when `JWT_SECRET` is missing or shorter than 32 bytes. The signing key is read only from this environment variable.

The server listens on port `8080` by default. Set `PORT` to use another port:

```bash
PORT=9090 go run ./cmd/api
```

Run the complete test suite, including the race detector:

```bash
go test ./...
go test -race ./...
```

### PostgreSQL mode

The application deliberately does not call GORM `AutoMigrate`. Schema changes are managed by the versioned SQL files in `migrations/` and applied by the separate `migrate` Compose service.

For the complete local stack, copy `.env.example` to `.env`, set a strong `JWT_SECRET`, and start the services:

```bash
cp .env.example .env
# Edit .env and set JWT_SECRET to at least 32 bytes.
docker compose up --build
```

Compose starts PostgreSQL with a persistent named volume, waits for its healthcheck, applies pending migrations, and then starts the API. To run it in the background:

```bash
docker compose up -d --build
docker compose logs -f migrate app
```

The database volume is preserved by a normal shutdown:

```bash
docker compose down
```

To intentionally delete the local database and rebuild it from all migrations:

```bash
docker compose down -v
docker compose up --build
```

Do not use `down -v` or migration `down` commands for production data. Production deployments should apply forward migrations only. The migration service can also be run explicitly before an application rollout:

```bash
docker compose run --rm migrate
docker compose up -d app
```

When running the API directly outside Docker, use the `DATABASE_URL` or `PG_*` variables shown in `.env.example` and apply migrations with the locally installed `migrate` CLI before starting the API. `.env` is not loaded automatically by Go:

```bash
set -a; source .env; set +a
migrate -database "${DATABASE_URL:-postgres://$PG_USER:$PG_PASSWORD@$PG_HOST:$PG_PORT/$PG_DB?sslmode=disable}" -path migrations up
go run ./cmd/api
```

The database connection uses a bounded connection pool and GORM warning-level logging with parameterized query output so credential hashes are not interpolated into SQL logs. `POST /api/v1/auth/register` creates a regular user; `GET /api/v1/users` lists users only for admins. New admin accounts must be created or promoted by a trusted operator after reviewing the required database operation. One bootstrap procedure is to register the account normally, run `UPDATE users SET role = 'admin' WHERE email = 'admin@example.com';` through a trusted database session, then log in again to obtain an API JWT carrying the new role. Browser authorization reads the current database role on each request.

Run a single API test stage:

```bash
go test ./internal/app -run TestStage2 -v
```

The staged API tests cover the health route and resource operations, validation and errors, updates and deletion, listing and filtering, then CORS and concurrent access.

## 4. REST API contract

All successful responses except `204 No Content` and all error responses use JSON. JSON field names use `snake_case`.

| Method and path | Success | Common errors |
|---|---|---|
| `GET /health` | `200` — `{"status":"ok"}` | — |
| `POST /api/v1/auth/register` | `201` — user and JWT | `422` invalid or duplicate registration |
| `POST /api/v1/auth/login` | `200` — user and JWT | `401` generic invalid credentials |
| `GET /api/v1/levels` | `200` — public array of levels | `400` invalid pagination |
| `GET /api/v1/levels/{id}` | `200` — public level | `400` invalid ID, `404` not found |
| `POST /api/v1/levels` | `201` — created level (admin) | `401` unauthenticated, `403` non-admin, `422` validation error |
| `PUT /api/v1/levels/{id}` | `200` — updated level (admin) | `401` unauthenticated, `403` non-admin, `404` not found |
| `DELETE /api/v1/levels/{id}` | `204` — empty body (admin) | `401` unauthenticated, `403` non-admin, `404` not found |
| `GET /api/v1/users` | `200` — array of users (admin) | `401` unauthenticated, `403` non-admin |
| `PUT /api/v1/users/{id}/role` | `200` — updated user (admin) | `401` unauthenticated, `403` non-admin, `422` invalid role |
| `POST /api/v1/games` | `201` — game for token owner | `401` unauthenticated, `409` duplicate, `422` invalid request |
| `GET /api/v1/games` | `200` — own games; admins see all | `401` unauthenticated |
| `GET /api/v1/games/{id}` | `200` — own game; admins see all | `401` unauthenticated, `404` not found/not owned |
| `POST /api/v1/games/{id}/move` | `200` — updated owned game | `401` unauthenticated, `404` not found/not owned, `422` invalid move |

Unknown routes return `404`; unsupported methods are rejected with a client error (`4xx`).

### Create a level

Level creation, update, and deletion require an admin JWT. Add `Authorization: Bearer $ADMIN_TOKEN` to those requests.

```bash
curl -i -X POST http://localhost:8080/api/v1/levels \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "title": "Forest Gate",
    "theme": "forest",
    "objective": "Find the hidden path",
    "difficulty": "medium",
    "move_limit": 100,
    "description": "Arrange the tiles to open the gate.",
    "columns": 4,
    "rows": 4,
    "time_limit": 300
  }'
```

### Get and list levels

```bash
curl http://localhost:8080/api/v1/levels/1
curl 'http://localhost:8080/api/v1/levels?page=1&limit=10&difficulty=medium'
```

### Update and delete a level

`PUT` replaces the resource. It uses the same input shape as `POST`; omitted optional fields are cleared and returned as `null`. The ID and `created_at` remain unchanged, while `updated_at` is refreshed. The API validates the ID first, then the body, then checks whether the level exists. An invalid request does not update stored data.

```bash
curl -i -X PUT http://localhost:8080/api/v1/levels/1 \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "title": "Forest Gate — Revised",
    "theme": "forest",
    "objective": "Find the hidden path",
    "difficulty": "hard"
  }'

curl -i -X DELETE http://localhost:8080/api/v1/levels/1 \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Users and games

Register and log in to receive a Bearer token. The game owner is derived from that token; `user_id` is not accepted in the create-game request. Users can access only their own games, while admins can inspect all games.

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"player@example.com","full_name":"Mystic Player","password":"correct-horse-42"}'

# Save the returned token as TOKEN, then:
level_id=1
curl -s -X POST http://localhost:8080/api/v1/games \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"level_id\":$level_id}"

curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/games
curl -X POST http://localhost:8080/api/v1/games/1/move \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"row":2,"column":1}'
```

Game creation starts with the ordered board, performs 1,000 legal random adjacent moves, and stores the resulting solvable board in `init`. The response also includes the derived current `board`. Move coordinates are zero-based, and a move must select a tile next to the empty cell (`0`).

### Validation and errors

A request with a missing, `null`, non-string, empty, or whitespace-only required field is rejected with `422`. A malformed or empty body, a JSON array instead of an object, or an optional field with an invalid type is also rejected with `422`. Validation errors can include field-specific details.

IDs in paths must be positive decimal integers. Invalid IDs return `400` with code `invalid_id`; valid but unknown IDs return `404` with code `not_found`.

List query parameters:

- `page` defaults to `1`; `limit` defaults to `10`.
- Both must be positive integers. Invalid values return `400` with code `invalid_pagination`.
- A `limit` greater than `100` is treated as `100`. Pages beyond the end return an empty array.
- Results are ordered by ascending ID. An empty collection is `[]`, not `null`.
- `difficulty` filters by exact, case-sensitive match. An empty filter value means no filter. Filtering is applied before pagination.

### Browser web UI

The browser UI is mounted alongside the JSON API by `internal/app/app.go`. It
uses the existing services directly; it does not make loopback HTTP requests to
`/api/v1`. Templates and local Bootstrap/HTMX assets are embedded in the
application binary.

| Method | Route | Access | Behavior |
|---|---|---|---|
| `GET` | `/` | public | Redirects guests to `/login` and authenticated users to `/game`. |
| `GET`, `POST` | `/login` | public | Render/login with a browser session cookie. |
| `GET`, `POST` | `/register` | public | Register a normal `user` and start a browser session. |
| `GET` | `/game` | authenticated | Render the three-panel game page. An optional `game_id` selects a user's game. |
| `POST` | `/game/start` | authenticated | Start or load a game for the session user. |
| `POST` | `/game/{id}/move` | authenticated | Apply one validated move to an owned game. |
| `GET` | `/levels` | admin | List levels and show the create form. |
| `POST` | `/levels` | admin | Create a level from the admin form. |
| `POST` | `/levels/{id}` | admin | Update only an existing level's title and description. |
| `GET` | `/users` | admin | List users without credential data. |
| `POST` | `/users/{id}/role` | admin | Change a user's role to `admin` or `user`. |
| `GET`, `POST` | `/logout` | public | Render the farewell page or expire the browser session. |
| `GET` | `/static/*path` | public | Serve embedded CSS and JavaScript assets. |

The root route uses a normal `303 See Other` redirect for browser requests. For
HTMX requests it returns `HX-Redirect` with a successful response. The same
`Vary: HX-Request` behavior is used wherever a route can return a complete page
or an HTMX fragment.

Browser authentication is separate from API Bearer authentication:

- Login and registration set `mystic_square_session` as an HttpOnly cookie with
  `Path=/`, `SameSite=Lax`, and a 24-hour lifetime.
- The cookie contains the signed token but is never rendered in HTML, exposed
  to JavaScript, or stored in browser storage.
- `WEB_COOKIE_SECURE=true` enables the `Secure` cookie attribute for HTTPS. The
  application also enables it when `APP_ENV` is `production` or `prod`.
- State-changing web forms use the HttpOnly `mystic_square_csrf` cookie and a
  matching form field or `X-CSRF-Token` header. Missing or invalid CSRF tokens
  are rejected.
- Browser sessions load the current user record on each request, so role
  changes take effect without trusting a stale role claim in the cookie token.

The shared header has separate menus for guests, regular users, and admins.
Admins see `Game`, `Levels`, `Users`, and `Logout`; regular users see `Game` and
`Logout`. The admin levels page presents the existing levels in a responsive
compact table. Only title and description controls are editable for existing
rows; the create form includes the fields required to define a new level. The
users page lists names, emails, roles, and creation dates, and provides only the
role update action.

The UI works with ordinary HTML forms when JavaScript is unavailable. When HTMX
is available, admin and game actions update server-rendered fragments while the
server remains authoritative for authorization and state.

Errors use this general shape and do not expose internal details:

```json
{
  "error": {
    "code": "not_found",
    "message": "level not found"
  }
}
```

The API uses codes including `invalid_id`, `invalid_pagination`, `validation_error`, `not_found`, `unauthorized`, `forbidden`, and `internal_server_error`. Login always returns the same `401` message (`invalid credentials`) for unknown emails and wrong passwords. A recovered panic becomes a generic `500` response; panic details are logged rather than returned to the client.

### CORS, concurrency, and storage

- Responses include `Access-Control-Allow-Origin`. An `OPTIONS` preflight returns `204` and advertises supported methods, including `POST`.
- The in-memory repository is safe for concurrent requests.
- Each call to `app.NewRouter()` gets a new, empty repository; resources are not shared between router instances and are lost when the process exits.
- When `DATABASE_URL` is set, `cmd/api` uses GORM repositories for levels, users, and games; PostgreSQL migrations must be applied before startup. JWT signing requires `JWT_SECRET` from the environment.
- `games` stores `init` and `moves` as JSONB. `UNIQUE (user_id, level_id)` enforces one game per user and level; tracking timestamps and terminal state support the browser game's elapsed-time and ordering views.

## 5. Implementation

The current release separates responsibilities into layers:

```text
cmd/api/main.go              starts the HTTP server and selects storage
internal/app/app.go          builds and configures the router
internal/model/              level, user, and game types and validation
internal/repository/         in-memory and GORM repositories plus DB setup
migrations/                  PostgreSQL schema managed by golang-migrate
internal/service/            level, user, and game business operations
internal/handler/            JSON API endpoints and request/response handling
internal/auth/               bcrypt authentication and HS256 JWT validation
internal/middleware/         recovery, request logging, CORS, authentication and RBAC
web/                         embedded HTML templates, CSS/JS assets, and browser handlers
```

The router is exposed as `app.NewRouter() http.Handler`; `app.NewRouterWithRepositories` is used when persistent repositories are configured. The application uses Gin and concurrency-safe in-memory stores by default, with optional GORM/PostgreSQL stores for all resources. The JSON API returns resources and the JSON error envelope shown above; the browser UI renders HTML through the separate `web.Handler` and session-cookie flow.

## 6. Development scope

Release 1 provides:

- Level creation, retrieval, listing, replacement, and deletion over HTTP.
- In-memory storage by default, or PostgreSQL persistence for levels, users, and games when `DATABASE_URL` is configured.
- Generated IDs and timestamps, level input validation, pagination, filtering, and deterministic list order.
- Public registration with bcrypt password hashing and login with signed 24-hour JWTs.
- Admin-only user listing and role updates; registration cannot set or elevate a role.
- Solvable game creation, current-board derivation, legal moves, win tracking, and the authenticated three-panel browser game page.
- Server-rendered login, registration, logout, and root routing pages using embedded Bootstrap and HTMX assets.
- Admin-only browser pages at `/levels` and `/users`, including level creation, title/description-only level edits, role updates, CSRF protection, responsive level tables, and separate admin/user navigation menus.
- HttpOnly browser sessions separate from API Bearer tokens, with current-role checks on protected web requests.
- Recovery, request logging, CORS, and isolation between router instances.
- PostgreSQL migrations and GORM models/repositories for `Level`, `User`, and `Game`.
- Database constraints for supported user roles and one game per user/level.

Authentication and authorization middleware protect all API game operations and admin-only mutations. The model layer exposes the admin rule (`User.CanManageLevels`), while JWT claims propagate the authenticated identity to API handlers. The web layer validates its session cookie separately and obtains the current role from the user repository. Game creation starts from solved order and performs 1,000 legal random adjacent moves, preserving solvability. Move coordinates are zero-based (`row`, `column`); for example, on a 3×3 board `row: 2, column: 1` selects tile 8 when the empty cell is to its right.

## 7. Lesson 8 Report

### Migration prompt

The migration prompt was adapted from the original Lesson 8 assignment to the Mystic Square domain:

```text
Generate PostgreSQL golang-migrate up/down migrations for the existing Level model
and new User and Game models. User has id, full_name and role (admin|user). Game
has id, level_id, user_id, init (JSONB board array), moves (JSONB array of [row,
column] pairs), and is_win BOOLEAN DEFAULT FALSE. Add explicit foreign keys,
indexes, CHECK(role IN ('admin','user')), UNIQUE(user_id, level_id), and reverse-order
down migrations. Use ON DELETE RESTRICT for level references and ON DELETE CASCADE
for user-owned games. Do not use GORM AutoMigrate.
```

The stored migrations include the level, user/game, authentication, default-level,
and game-tracking migrations under `migrations/`. `JSONB` was chosen instead of a
fixed-size PostgreSQL array because level dimensions are configurable and moves are
pairs of coordinates.

### N+1 proof

The original Release 1 implementation used an in-memory level repository, so it had
no SQL N+1 baseline to measure. It would be misleading to invent a BEFORE latency or
terminal log. The new database path avoids introducing that baseline: game list
loading is implemented in `internal/repository/game.go` with
`Preload("Level").Preload("User")`, which produces the main query plus batched
association queries rather than one query per game.

Capture project-specific measurements after starting PostgreSQL and adding a game-list
endpoint or integration harness. Record the actual output in this table:

| Variant | Expected SQL shape | SELECT count | Latency |
|---|---|---:|---:|
| BEFORE (deliberately naive comparison) | main query + one `levels` and one `users` query per game | not run in Release 1 | not measured |
| AFTER (`GORMGameRepository.List`) | main query + batched `IN (...)` preload queries | constant with respect to game count | not measured |

The relevant AFTER code is:

```go
r.db.WithContext(ctx).
    Preload("Level").
    Preload("User").
    Order("id ASC").
    Find(&games)
```

### Roles and game uniqueness

`model.User.CanManageLevels` expresses the admin rule, while authentication middleware
must provide the identity before HTTP authorization can enforce it. The database
constraint `UNIQUE (user_id, level_id)` enforces the one-game-per-user-per-level rule
even when multiple requests race.

## 8. Assignment workflow and submission

1. Confirm that the root `VARIANT` file contains `levels`.
2. Run `go test ./...` while implementing the service; use the staged tests to narrow down failures.
3. Implement and explain the code behind `app.NewRouter()`. The tests exercise the API through this handler.
4. Run `gofmt`, `go vet ./...`, and `go test ./...` before submission. Use `go test -race ./...` to check concurrent behavior.
5. Submit the repository with passing checks and a short explanation of the implementation and design decisions. Be prepared to explain the code and make a small requirement change during review.

The original assignment rubric allocates points to visible tests, hidden tests, formatting and vet checks, code quality, and the code review. Follow the deadline and review instructions provided by the instructor.
