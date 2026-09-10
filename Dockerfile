# ---- 构建阶段 ----
FROM golang:1.26-alpine AS builder

WORKDIR /src

# 先复制依赖清单，利用 Docker 层缓存
COPY go.mod go.sum ./
RUN go mod download

# 再复制源码编译（CGO 关闭，纯静态二进制）
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /shiken .

# ---- 运行阶段 ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 1000 shiken

WORKDIR /app
COPY --from=builder /shiken /app/shiken

# 备份文件写入目录
RUN mkdir -p /app/backups && chown -R shiken:shiken /app
USER shiken

EXPOSE 8080

# MySQL 连接串通过环境变量注入，例如：
#   docker run -e SHIKEN_MYSQL_DSN="root:123456@tcp(100.108.142.7:3306)/shiken?charset=utf8mb4&multiStatements=true" -p 8080:8080 shiken
ENTRYPOINT ["/app/shiken"]
