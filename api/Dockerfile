# ===== BUILD STAGE =====
FROM golang:1.23-alpine AS build
WORKDIR /app
ENV GOTOOLCHAIN=auto

# 1) Cache deps theo go.mod (và go.sum nếu có)
COPY go.mod ./
# COPY go.sum ./
RUN go mod download

# 2) Copy source
COPY . .

# 3) Gen go.sum + indirect deps
RUN go mod tidy

# 4) Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o app ./main.go


# ===== RUNTIME STAGE =====
FROM alpine:3.20
# Cài psql client để chạy migrate.sh
RUN apk add --no-cache postgresql16-client ca-certificates

WORKDIR /app

# Binary
COPY --from=build /app/app /app/app

# Copy script + migrations (an toàn dù bạn còn mount volume tại compose)
COPY db /app/db
RUN chmod +x /app/db/migrate.sh

EXPOSE 8080

# Lưu ý: lệnh chạy thực tế lấy từ docker-compose.yml:
# command: ["/bin/sh", "-c", "/app/db/migrate.sh && ./app"]
# nên không cần CMD ở đây. Nếu muốn chạy độc lập, có thể bật CMD bên dưới.
# CMD ["./app"]
