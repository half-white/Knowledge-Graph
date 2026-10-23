
# ============================================================
# SSE 知识图谱后端镜像
# 构建上下文 = 仓库根目录（由 deploy/sse-app/docker-compose.yml 指定）
# ============================================================

# ---------- 构建阶段 ----------
FROM golang:1.23 AS build
WORKDIR /src

# 先拷贝依赖清单与 vendor，最大化利用构建缓存
COPY go.mod go.sum ./
COPY vendor ./vendor

# 拷贝源码（.dockerignore 已排除 node_modules/dist/logs 等）
COPY . .

# 纯静态编译（vendor 目录随仓库提交，构建不需要外网拉依赖）
ENV CGO_ENABLED=0 GOFLAGS=-mod=vendor GOTOOLCHAIN=local
RUN go build -trimpath -ldflags="-s -w" -o /out/SSE .

# ---------- 运行阶段 ----------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# 可执行文件
COPY --from=build /out/SSE /app/SSE
# 供 getProjectRoot()/findProjectRoot() 定位项目根（以 go.mod 为锚点）
COPY go.mod /app/go.mod

# 运行期可写目录（默认由 compose 用 volume 覆盖为持久化目录）。
# 注意：resource/ 不入库（git 不追踪空目录，上传文件属运行期数据），
# 不能用 COPY resource 拷入，否则干净克隆的 CI 构建会因目录缺失而失败。
RUN mkdir -p /app/logs /app/resource/doc

# 默认容器化运行参数（数据库地址/监听地址均可被 compose 环境变量覆盖）
ENV SSE_ADDR=:8080 \
    SSE_LOG_CONSOLE=true \
    SSE_LOG_FILE=true \
    SSE_LOG_DIR=/logs

EXPOSE 8080
CMD ["/app/SSE"]
