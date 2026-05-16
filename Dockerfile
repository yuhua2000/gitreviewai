# Stage 1: Build frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app/internal/api/frontend
COPY internal/api/frontend/package.json internal/api/frontend/package-lock.json* ./
RUN npm install
COPY internal/api/frontend/ ./
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.25-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go env -w GOPROXY=https://goproxy.cn,direct && go mod download
COPY . .
COPY --from=frontend-builder /app/internal/api/frontend/dist ./internal/api/frontend/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o gitreviewai cmd/server/main.go

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
