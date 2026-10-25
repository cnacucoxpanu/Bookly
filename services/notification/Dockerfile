FROM golang:1.27-alpine AS build

WORKDIR /src
COPY contracts ./contracts

WORKDIR /src/services/notification
COPY services/notification/go.mod services/notification/go.sum ./
RUN go mod download

COPY services/notification ./
RUN CGO_ENABLED=0 go build -o /out/notification ./cmd/server

FROM alpine:3.22

RUN adduser -D -H app
USER app

COPY --from=build /out/notification /usr/local/bin/notification
ENTRYPOINT ["notification"]
