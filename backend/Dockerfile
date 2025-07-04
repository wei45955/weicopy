# 确保使用支持CGO的基础镜像（如加装gcc的alpine）
FROM golang:1.21-alpine AS builder

# 安装SQLite3依赖（关键步骤）
RUN apk add --no-cache gcc musl-dev

# 设置工作目录
WORKDIR /app

# 安装依赖
RUN apk add --no-cache git

# 复制go.mod和go.sum
COPY go.mod ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o weicopy .

# 最终镜像
FROM alpine:latest

WORKDIR /app

# 安装运行时依赖
RUN apk --no-cache add ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

# 从构建阶段复制二进制文件
COPY --from=builder /app/weicopy .

# 创建必要的目录
RUN mkdir -p /app/data /app/uploads

# 设置环境变量
ENV GIN_MODE=release
ENV PORT=8081
ENV DB_PATH=/app/data/weicopy.db
ENV UPLOAD_PATH=/app/uploads
ENV MAX_UPLOAD_SIZE_MB=50
ENV ENABLE_REGISTRATION=false

EXPOSE 8081

CMD ["./weicopy"]