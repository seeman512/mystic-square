# Mystic Square — Levels Project

Mystic Square is a browser-based sliding-puzzle game built around the classic 15 puzzle. This repository contains a REST API for creating, listing, retrieving, updating, and deleting level configurations, plus default users and game sessions. It also contains the PostgreSQL persistence layer: GORM mappings and migrations for levels, users, and game sessions.

The HTTP API currently exposes level management, default-user creation/listing, and game sessions. Authentication and the browser game are not implemented yet; user and game endpoints use the selected user/level IDs directly.

## 1. Project overview

Players solve a sliding puzzle by moving numbered tiles into the empty cell until the board is in its solved order. A level configures a puzzle's theme, objective, difficulty, board dimensions, and optional limits.

The product defines two roles:

| Role | Intended permissions |
|---|---|
| `admin` | Browse and play levels; create levels. |
| `user` | Browse and play levels; cannot create levels. |

The server must enforce these permissions when authentication and authorization are added. Release 1 does **not** implement accounts, authentication, role checks, or a browser UI; its API currently allows level creation without authentication.

### Puzzle rules and player flow

- A board with `rows × columns` cells contains `rows × columns - 1` numbered tiles and one empty cell. A standard 15 puzzle uses a 4×4 board and tiles 1–15.
- A legal move slides a tile horizontally or vertically adjacent to the empty cell into that cell. Diagonal moves are not allowed.
- The solved arrangement orders the tiles from left to right and top to bottom, with the empty cell last.
- A game session should start with a solvable arrangement and track the board, move count, and elapsed time separately from the level definition.
- A player selects a level, starts a game, makes legal moves, and sees whether they solved the puzzle or reached an enabled time or move limit.

### Creating a level

In the complete product, an admin enters the level's required details and optional description and move limit. The application validates the input, assigns system-managed identifiers and timestamps, and makes the level available to play. Users may play levels but must not create them through either the interface or API once role-based access is implemented.

## 2. Level resource and fields

The `levels` variant is defined by `internal/app/variants_test.go`. Its four required string fields, optional integer, and optional text field are:

| Field | Type | Request requirement | Purpose |
|---|---|---|---|
| `title` | string | Required | Name shown in the level list and game. |
| `theme` | string | Required | Setting or theme, such as a forest or space station. |
| `objective` | string | Required | What the player must accomplish. |
| `difficulty` | string | Required | Difficulty label; this is also the list-filter field. The project defines `low`, `medium`, or `hard` as its difficulty values. Release 1 currently checks that this field is not blank but does not enforce the enum. |
| `move_limit` | integer | Optional | Maximum permitted moves; `-1` means unlimited. |
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
| `updated_at` | timestamp | Assigned by the service | Creation time, refreshed when the resource is updated. |

`move_limit` may be from `-1` through `10000`; only `-1` means unlimited, while `0` is a configured limit. `time_limit` follows the same unlimited convention. An omitted limit is represented as `null` in the resource, and is treated as unlimited by the level model.

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

Use Go 1.27.1. Without `DATABASE_URL`, the API uses an isolated in-memory level repository, which is useful for local development and the existing HTTP tests. Start the HTTP API from the repository root:

```bash
go run ./cmd/api
```

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

The Lesson 8 database path is selected by setting `DATABASE_URL`, or by exporting the `PG_*` variables shown in `.env.example`. The application deliberately does not call GORM `AutoMigrate`.

```bash
# For the local Docker database:
docker compose up -d postgres
set -a; source .env.example; set +a

# Or provide DATABASE_URL directly for an existing PostgreSQL server:
# export DATABASE_URL='postgres://user:password@localhost:5432/database?sslmode=disable'

migrate -database "${DATABASE_URL:-postgres://$PG_USER:$PG_PASSWORD@$PG_HOST:$PG_PORT/$PG_DB?sslmode=disable}" -path migrations up
go run ./cmd/api
```

The database connection uses a bounded connection pool and GORM SQL logging at `Info` level for local diagnostics. `.env` is not loaded automatically by Go; export it with `set -a; source .env; set +a` before starting the API. The `users` and `games` models/repositories back the HTTP endpoints. `POST /api/v1/users` creates a default regular user, and `GET /api/v1/users` lists users.

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
| `POST /api/v1/levels` | `201` — created level | `422` validation error |
| `GET /api/v1/levels` | `200` — array of levels | `400` invalid pagination |
| `GET /api/v1/levels/{id}` | `200` — level | `400` invalid ID, `404` not found |
| `PUT /api/v1/levels/{id}` | `200` — updated level | `400` invalid ID, `422` validation error, `404` not found |
| `DELETE /api/v1/levels/{id}` | `204` — empty body | `400` invalid ID, `404` not found |
| `POST /api/v1/users` | `201` — default user | `500` internal error |
| `GET /api/v1/users` | `200` — array of users | `500` internal error |
| `POST /api/v1/games` | `201` — shuffled game | `409` duplicate game, `422` invalid request |
| `GET /api/v1/games` | `200` — array of games | `500` internal error |
| `GET /api/v1/games/{id}` | `200` — game with current board | `400` invalid ID, `404` not found |
| `POST /api/v1/games/{id}/move` | `200` — updated game | `404` not found, `422` invalid move |

Unknown routes return `404`; unsupported methods are rejected with a client error (`4xx`).

### Create a level

```bash
curl -i -X POST http://localhost:8080/api/v1/levels \
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
  -H 'Content-Type: application/json' \
  -d '{
    "title": "Forest Gate — Revised",
    "theme": "forest",
    "objective": "Find the hidden path",
    "difficulty": "hard"
  }'

curl -i -X DELETE http://localhost:8080/api/v1/levels/1
```

### Users and games

Create a default regular user and use its ID with a selected level to start a game:

```bash
user_id=$(curl -s -X POST http://localhost:8080/api/v1/users | jq -r .id)
level_id=1
curl -s -X POST http://localhost:8080/api/v1/games \
  -H 'Content-Type: application/json' \
  -d "{\"user_id\":$user_id,\"level_id\":$level_id}"

curl http://localhost:8080/api/v1/users
curl http://localhost:8080/api/v1/games
curl -X POST http://localhost:8080/api/v1/games/1/move \
  -H 'Content-Type: application/json' -d '{"row":2,"column":1}'
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

Errors use this general shape and do not expose internal details:

```json
{
  "error": {
    "code": "not_found",
    "message": "level not found"
  }
}
```

The API uses codes including `invalid_id`, `invalid_pagination`, `validation_error`, `not_found`, and `internal_server_error`. A recovered panic becomes a generic `500` response; panic details are logged rather than returned to the client.

### CORS, concurrency, and storage

- Responses include `Access-Control-Allow-Origin`. An `OPTIONS` preflight returns `204` and advertises supported methods, including `POST`.
- The in-memory repository is safe for concurrent requests.
- Each call to `app.NewRouter()` gets a new, empty repository; resources are not shared between router instances and are lost when the process exits.
- When `DATABASE_URL` is set, `cmd/api` uses GORM repositories for levels, users, and games; PostgreSQL migrations must be applied before startup.
- `games` stores `init` and `moves` as JSONB. `UNIQUE (user_id, level_id)` enforces one game per user and level; game list loading preloads both `Level` and `User` to avoid N+1 queries.

## 5. Implementation

The current release separates responsibilities into layers:

```text
cmd/api/main.go              starts the HTTP server and selects storage
internal/app/app.go          builds and configures the router
internal/model/              level, user, and game types and validation
internal/repository/         in-memory and GORM repositories plus DB setup
migrations/                  PostgreSQL schema managed by golang-migrate
internal/service/            level business operations
internal/handler/            HTTP endpoints and request/response handling
internal/middleware/         recovery, request logging, and CORS
```

The router is exposed as `app.NewRouter() http.Handler`; `app.NewRouterWithRepositories` is used when persistent repositories are configured. The API uses Gin and concurrency-safe in-memory stores by default, with optional GORM/PostgreSQL stores for all resources. Successful resource operations return the resource itself; errors use the JSON error envelope shown above.

## 6. Development scope

Release 1 provides:

- Level creation, retrieval, listing, replacement, and deletion over HTTP.
- In-memory storage by default, or PostgreSQL persistence for levels, users, and games when `DATABASE_URL` is configured.
- Generated IDs and timestamps, level input validation, pagination, filtering, and deterministic list order.
- Default-user creation and listing over HTTP.
- Solvable game creation, current-board derivation, legal moves, and win tracking.
- Recovery, request logging, CORS, and isolation between router instances.
- PostgreSQL migrations and GORM models/repositories for `Level`, `User`, and `Game`.
- Database constraints for supported user roles and one game per user/level.

Authentication, authorization middleware, and the browser UI remain outside the current HTTP release. The model layer exposes the admin rule (`User.CanManageLevels`), while enforcement at the HTTP boundary requires authentication identity propagation. Game creation starts from solved order and performs 1,000 legal random adjacent moves, preserving solvability. Move coordinates are zero-based (`row`, `column`); for example, on a 3×3 board `row: 2, column: 1` selects tile 8 when the empty cell is to its right.

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

The stored migrations are `migrations/000001_create_levels.*.sql` and
`migrations/000002_create_users_and_games.*.sql`. `JSONB` was chosen instead of a
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
