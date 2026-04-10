FROM golang:1.23 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/sensimul ./cmd/sensimul

FROM alpine:3.20
RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /out/sensimul /usr/local/bin/sensimul
COPY config/sensimul.yaml /app/config/sensimul.yaml

ENTRYPOINT ["sensimul"]
CMD ["run", "--config", "/app/config/sensimul.yaml"]
