FROM golang:1.26.1-bookworm AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /app ./cmd/http-api/

FROM gcr.io/distroless/static-debian12

COPY --from=builder /app /app

ENV REPOS_DIR=/data/repos

EXPOSE 8080

ENTRYPOINT ["/app"]
