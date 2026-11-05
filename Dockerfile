FROM golang:1.24-alpine AS build

WORKDIR /src
RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/bot ./cmd/bot

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
  && adduser -D -u 1000 bot \
  && mkdir -p /data \
  && chown bot:bot /data

COPY --from=build /out/bot /bot

USER bot
WORKDIR /home/bot

ENV DATABASE_PATH=/data/kbju.db
ENV TIMEZONE=Europe/Moscow

VOLUME ["/data"]
ENTRYPOINT ["/bot"]
