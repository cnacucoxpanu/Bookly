# Bookly

Coworking space booking platform built as three Go microservices.

- **Auth** – registration, login, JWT access tokens, user lookup for other services.
- **Booking** – coworking spaces and bookings. Protects against double booking under concurrent requests.
- **Notification** – listens to booking events and keeps a log of notifications.

## Architecture

```
                  REST                         REST
   client ───────────────▶  Auth        client ───────────────▶  Booking
                             │                                    │   │
                             │ PostgreSQL (auth)                  │   │ PostgreSQL (booking)
                             │                                    │   │
                             │◀──── gRPC: ValidateToken ──────────┘   │
                             │                                        │ publish
                             │                                        ▼
                             │                                   RabbitMQ
                             │◀──── gRPC: GetUser ─┐          (topic exchange
                             │                     │           bookly.events)
                                                Notification ◀────────┘
                                                   │ consume booking.*
                                                   ▼
                                           PostgreSQL (notification)
```

- Every service has its own database and its own Clean Architecture layers:
  `handler → service → repository`. The service layer knows nothing about HTTP or Postgres.
- Booking checks tokens by calling Auth over **gRPC**, so it never needs the JWT secret.
- Booking publishes `booking.created` and `booking.cancelled` events to **RabbitMQ**
  (persistent messages, publisher confirms). Notification consumes them from a durable queue
  and handles each event idempotently, because RabbitMQ delivers at least once.
- A failing broker never breaks a booking: the event is logged and the booking stays saved.

## Stack

Go, PostgreSQL (pgx), gRPC / Protocol Buffers, RabbitMQ, JWT + bcrypt,
OpenAPI / Swagger UI, Docker and Docker Compose, golang-migrate.

## Running

Requirements: Docker with Compose.

Create a `.env` file in the project root:

```
POSTGRES_PASSWORD=choose_a_password
JWT_SECRET=a_random_string_of_at_least_32_characters

# optional, these are the defaults
POSTGRES_USER=bookly
POSTGRES_DB=bookly
POSTGRES_PORT=5432
RABBITMQ_USER=bookly
RABBITMQ_PASSWORD=bookly
```

Avoid special characters in passwords, they end up inside connection URLs.

Then start everything:

```
docker compose up -d --build
```

| Service      | URL                                  |
|--------------|--------------------------------------|
| Auth         | http://localhost:8081                |
| Booking      | http://localhost:8082                |
| RabbitMQ UI  | http://localhost:15672               |
| Swagger UI   | `/swagger/` on the Auth and Booking ports |

Migrations are applied by each service on startup. Booking comes with a few seeded spaces.

## Try it

```
# register and log in
curl -X POST localhost:8081/register -H "Content-Type: application/json" \
  -d '{"email":"anna@example.com","password":"password123","name":"Anna"}'

curl -X POST localhost:8081/login -H "Content-Type: application/json" \
  -d '{"email":"anna@example.com","password":"password123"}'
# -> {"access_token":"<token>", ...}

# look at the spaces and book one
curl localhost:8082/spaces

curl -X POST localhost:8082/bookings \
  -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
  -d '{"space_id":1,"start_time":"2030-01-15T10:00:00Z","end_time":"2030-01-15T12:00:00Z"}'
```

The same request again returns `409 Conflict`. Watch the notification service log to see the
event arrive:

```
docker compose logs -f notification
```

Creating spaces is allowed for admins only. There is no endpoint to promote a user, so change
the role in the database and log in again (the role is stored in the token):

```
docker compose exec postgres sh -c 'psql -U "$POSTGRES_USER" -d auth'
```

```sql
UPDATE users SET role = 'admin' WHERE email = 'anna@example.com';
```

## API overview

Auth (`:8081`)

| Method | Path        | Description                         |
|--------|-------------|-------------------------------------|
| POST   | `/register` | create a user                       |
| POST   | `/login`    | get an access token                 |
| GET    | `/me`       | user id and role from the token     |

Booking (`:8082`)

| Method | Path             | Description                              |
|--------|------------------|------------------------------------------|
| GET    | `/spaces`        | list spaces                              |
| GET    | `/spaces/{id}`   | get a space                              |
| POST   | `/spaces`        | create a space (admin)                   |
| POST   | `/bookings`      | book a space                             |
| GET    | `/bookings`      | my bookings                              |
| GET    | `/bookings/{id}` | get a booking (owner or admin)           |
| DELETE | `/bookings/{id}` | cancel a booking (owner or admin)        |

Full descriptions are in `services/*/docs/openapi.yaml` and in Swagger UI.

## How double booking is prevented

Two requests for the same slot at the same moment must never both succeed. Booking uses two layers:

1. **Transaction with `SERIALIZABLE` isolation.** The repository checks for overlapping active
   bookings and inserts the new one in a single transaction. If two transactions race,
   Postgres aborts one with a serialization failure (`40001`). The repository retries it, and
   the second attempt sees the winner's booking and answers `409`.
2. **Exclusion constraint in the schema.**
   `EXCLUDE USING gist (space_id WITH =, tstzrange(start_time, end_time) WITH &&) WHERE (status = 'active')`
   makes overlapping active bookings impossible even if the application code is bypassed.
   Back-to-back bookings (one ends when the next starts) are allowed, cancelled bookings free the slot.

## Tests

Unit tests (no external dependencies):

```
cd services/auth && go test ./...
cd services/booking && go test ./...
cd services/notification && go test ./...
```

Concurrency tests for Booking need a real PostgreSQL with the `booking` database, for example
the one from Docker Compose:

```
TEST_DATABASE_URL=postgres://bookly:<password>@localhost:5432/booking \
  go test ./internal/repository/ -v
```

They fire many parallel requests at the same slot and check that exactly one wins and that no
two active bookings ever overlap.

## Project layout

```
contracts/              shared module: .proto files, generated gRPC code, event format
deploy/postgres/        creates a database per service on first start
services/auth/          cmd/server, internal/{handler,service,repository,grpcserver,...}, migrations, docs
services/booking/       same layout, plus internal/{authclient,messaging}
services/notification/  same layout, plus internal/consumer
docker-compose.yml
```

Regenerating the gRPC code after editing a `.proto` file needs `protoc`, `protoc-gen-go` and
`protoc-gen-go-grpc`:

```
sh contracts/generate.sh
```

## Possible improvements

- Transactional outbox for events, so a crash between the commit and the publish cannot lose one.
- Refresh tokens and an endpoint to manage roles.
- Dead-letter queue for messages that cannot be processed.
