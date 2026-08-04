# syntax=docker/dockerfile:1

# ---- 构建阶段 ----
# mattn/go-sqlite3 依赖 CGO，需要 gcc/musl-dev 工具链；产物为动态链接 musl 的二进制
FROM golang:1.26-alpine AS build

RUN apk add --no-cache gcc musl-dev

WORKDIR /src

# 依赖代理：默认国内可达的 goproxy.cn（proxy.golang.org 在部分网络环境不可达）
# 覆盖：docker build --build-arg GOPROXY=https://proxy.golang.org,direct .
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/qtcloud-auth ./cmd/server

# ---- 运行阶段 ----
# 与构建阶段同为 musl libc，二进制可直接运行
FROM alpine:3.20

RUN addgroup -S app && adduser -S -G app app
COPY --from=build /out/qtcloud-auth /usr/local/bin/qtcloud-auth

USER app
EXPOSE 8080

# 监听地址由环境变量 LISTEN_ADDR 控制（默认 :8080，对齐 FC custom-container port）
ENTRYPOINT ["qtcloud-auth"]
