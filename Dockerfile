FROM golang:1.23 AS builder

ARG APP_NAME=sensimul

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/app ./cmd/${APP_NAME}

FROM alpine:3.20
RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /out/app /usr/local/bin/app
COPY config/sensimul.yaml /app/config/sensimul.yaml

ENTRYPOINT ["app"]
CMD ["run", "--config", "/app/config/sensimul.yaml"]
