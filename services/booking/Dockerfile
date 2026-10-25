FROM golang:1.27-alpine AS build

WORKDIR /src
COPY contracts ./contracts

WORKDIR /src/services/booking
COPY services/booking/go.mod services/booking/go.sum ./
RUN go mod download

COPY services/booking ./
RUN CGO_ENABLED=0 go build -o /out/booking ./cmd/server

FROM alpine:3.22

RUN adduser -D -H app
USER app

COPY --from=build /out/booking /usr/local/bin/booking
ENTRYPOINT ["booking"]
