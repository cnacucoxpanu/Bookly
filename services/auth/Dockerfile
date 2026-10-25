FROM golang:1.27-alpine AS build

WORKDIR /src
COPY contracts ./contracts

WORKDIR /src/services/auth
COPY services/auth/go.mod services/auth/go.sum ./
RUN go mod download

COPY services/auth ./
RUN CGO_ENABLED=0 go build -o /out/auth ./cmd/server

FROM alpine:3.22

RUN adduser -D -H app
USER app

COPY --from=build /out/auth /usr/local/bin/auth
ENTRYPOINT ["auth"]
