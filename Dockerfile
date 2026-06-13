# Stage 1: Build frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.25-alpine AS go-builder
RUN apk --no-cache add git
WORKDIR /app
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download
COPY . .
COPY --from=frontend-builder /app/dist ./frontend/dist
RUN VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev") \
    && COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-s -w -X github.com/yuhua2000/gitreviewai/cmd.version=${VERSION} -X github.com/yuhua2000/gitreviewai/cmd.commit=${COMMIT}" \
    -o gitreviewai .

# Stage 3: Runtime
FROM alpine:latest
RUN apk --no-cache add ca-certificates
RUN adduser -D -s /bin/sh gitreviewai
WORKDIR /app
COPY --from=go-builder /app/gitreviewai .
RUN mkdir -p /app/data && chown -R gitreviewai:gitreviewai /app
USER gitreviewai
EXPOSE 8080
ENTRYPOINT ["./gitreviewai"]
CMD ["server"]
