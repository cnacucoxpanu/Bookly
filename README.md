# Bookly

A small coworking space booking backend I wrote in Go to practice microservices.
You can register, look at the available spaces, book one for a time slot and cancel the booking.

It is split into three services:

- **auth** – registration, login (JWT, bcrypt) and a gRPC API the other services use to check tokens
- **booking** – spaces and bookings, REST API
- **notification** – listens to booking events from RabbitMQ and saves a notification log

Each service has its own PostgreSQL database and is split into handler / service / repository layers.

## How the services talk to each other

- The client talks to auth and booking over REST.
- Booking doesn't know the JWT secret. For every protected request it asks auth over gRPC
  whether the token is valid.
- When a booking is created or cancelled, booking sends an event to RabbitMQ.
  Notification reads it, asks auth for the user's name and email over gRPC and writes a line to its database.
- If RabbitMQ is down the booking still goes through, the failed event is only logged.
  (A proper fix would be an outbox table, I didn't do it.)

## Running it

You need Docker. Create a `.env` file in the root folder:

```
POSTGRES_PASSWORD=some_password
JWT_SECRET=any_random_string_at_least_32_characters_long
```

Then:

```
docker compose up -d --build
```

Auth is on http://localhost:8081, booking on http://localhost:8082.
Swagger UI is at `/swagger/` on both ports, RabbitMQ management is on http://localhost:15672
(user and password `bookly`). Tables are created automatically on startup and booking
comes with a few spaces already added.

Quick try:

```
curl -X POST localhost:8081/register -H "Content-Type: application/json" \
  -d '{"email":"anna@example.com","password":"password123","name":"Anna"}'

curl -X POST localhost:8081/login -H "Content-Type: application/json" \
  -d '{"email":"anna@example.com","password":"password123"}'

curl localhost:8082/spaces

curl -X POST localhost:8082/bookings \
  -H "Authorization: Bearer <access_token from login>" -H "Content-Type: application/json" \
  -d '{"space_id":1,"start_time":"2030-01-15T10:00:00Z","end_time":"2030-01-15T12:00:00Z"}'
```

Send the last request twice and the second one gets `409`.
`docker compose logs -f notification` shows the notification for the first one.

Only admins can create new spaces. There is no endpoint for making someone an admin,
so I change the role in the database by hand (`UPDATE users SET role = 'admin' ...`
in the `auth` database) and log in again, because the role is stored in the token.

## Double booking

The part I spent the most time on. Two people booking the same slot at the same moment
must not both succeed, so there are two protections:

1. The check for overlapping bookings and the insert happen in one transaction with
   `SERIALIZABLE` isolation. If two transactions collide, Postgres aborts one of them,
   the code retries it and the second try already sees the other booking and returns `409`.
2. The `bookings` table has an exclusion constraint on (space, time range), so the database
   itself refuses overlapping active bookings even if the Go code has a bug.

There are tests that send 20–30 parallel requests for the same slot and check that exactly
one of them wins. They need a real Postgres, see below.

## Tests

```
cd services/auth && go test ./...
cd services/booking && go test ./...
cd services/notification && go test ./...
```

The tests in `services/booking/internal/repository` are skipped unless you point them
to a database:

```
TEST_DATABASE_URL=postgres://bookly:<password>@localhost:5432/booking go test ./internal/repository/
```

(for example the one from docker compose, the `booking` database is created there automatically)

## Folders

```
contracts/   .proto file, generated gRPC code, the event struct shared by booking and notification
deploy/      script that creates a database for every service
services/    auth, booking, notification
```

If you change `contracts/proto/auth/v1/auth.proto`, run `sh contracts/generate.sh`
(needs `protoc`, `protoc-gen-go` and `protoc-gen-go-grpc`).

## What could be better

- outbox for the events
- refresh tokens
- a dead letter queue in RabbitMQ for messages that can't be processed
- a way to manage roles without touching the database
