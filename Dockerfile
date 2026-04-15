FROM golang:1.26.1-bookworm AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /app ./cmd/http-api/main.go

FROM gcr.io/distroless/static-debian12

WORKDIR /srv

COPY --from=builder /app /srv/app
COPY --from=builder /build/scripts /srv/scripts

ENV REPOS_DIR=/data/repos

EXPOSE 8080

ENTRYPOINT ["/srv/app", "/data/repos"]
