
# ============================================================
# SSE 知识图谱前端镜像（Vue3 + Vite）
# 构建上下文 = 仓库根目录（由 deploy/sse-app/docker-compose.yml 指定）
# 产物：静态资源 dist + nginx 反向代理 /api -> backend
# ============================================================

# ---------- 构建阶段 ----------
FROM node:20-alpine AS build
WORKDIR /build

# 先拷贝 lock 文件，最大化 npm ci 缓存
COPY vue/sse/package.json vue/sse/package-lock.json ./
RUN npm ci --no-audit --no-fund

COPY vue/sse ./
RUN npm run build

# ---------- 运行阶段 ----------
FROM nginx:1.27-alpine
COPY deploy/sse-app/web/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /build/dist /usr/share/nginx/html

EXPOSE 80
