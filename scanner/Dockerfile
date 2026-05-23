FROM golang:1.24-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o hakato .

# ── Runtime image ──────────────────────────────────────
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/hakato        ./hakato
COPY --from=builder /app/all_tokens.json ./all_tokens.json
COPY --from=builder /app/output.json   ./output.json

EXPOSE 8080
CMD ["./hakato"]
