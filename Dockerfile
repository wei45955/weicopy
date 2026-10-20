FROM golang:1.24-alpine AS builder

WORKDIR /src
COPY backend/go.mod backend/go.sum ./
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=$GOPROXY
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/weicopy .

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /out/weicopy /app/weicopy
COPY frontend /app/static
RUN mkdir -p /app/data /app/uploads

ENV GIN_MODE=release \
    PORT=8080 \
    DB_PATH=/app/data/weicopy.db \
    UPLOAD_PATH=/app/uploads \
    STATIC_DIR=/app/static \
    MAX_UPLOAD_SIZE_MB=50

EXPOSE 8080
CMD ["/app/weicopy"]
