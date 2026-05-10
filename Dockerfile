# 使用官方Go镜像作为构建环境
FROM golang:1.22-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制go mod和sum文件
COPY go.mod go.sum ./

# 配置国内代理并下载依赖
RUN go env -w GOPROXY=https://goproxy.cn,direct && \
    go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o gitreviewai cmd/server/main.go

# 使用轻量级的Alpine镜像作为运行环境
FROM alpine:latest

# 安装ca-certificates以支持HTTPS请求
RUN apk --no-cache add ca-certificates

# 创建非root用户
RUN adduser -D -s /bin/sh gitreviewai

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/gitreviewai .

# 复制配置文件模板（如果需要）
# COPY config.yaml .

# 更改文件所有者
RUN chown -R gitreviewai:gitreviewai /app

# 切换到非root用户
USER gitreviewai

# 暴露端口
EXPOSE 8080

# 运行应用
ENTRYPOINT ["./gitreviewai"]