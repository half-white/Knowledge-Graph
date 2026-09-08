# 一键部署（Docker 化应用栈）

把 SSE 知识图谱「后端 + 前端」容器化，由 `docker compose` 统一托管，代码里数据库地址/监听地址均已改为**环境变量可覆盖**（默认值不变，宿主机原生运行不受影响）。

## 组成

| 服务 | 说明 | 对外端口 |
|------|------|---------|
| `backend` | Go(gin) 后端，镜像 `sse/backend` | 8080 |
| `web` | nginx 托管 Vue 前端 dist，`/api` 反代到 backend | 8090 |

数据库 Neo4j(7687)、MySQL/MariaDB(3306) 仍由你现有的容器提供，后端通过 `host.docker.internal` 访问宿主已发布端口。

## 目录

```
deploy/sse-app/
├── docker-compose.yml     # 应用栈编排（compose 项目名 sse）
├── .env.example           # 环境变量样例（复制为 .env 使用）
├── backend/Dockerfile     # 后端镜像：golang:1.23 多阶段 → alpine
└── web/
    ├── Dockerfile         # 前端镜像：node 构建 → nginx
    └── nginx.conf         # 静态托管 + /api 反代 backend:8080
```

## 快速上手

```bash
cd /Users/xieenping/work/Knowledge-Graph

# 1) 构建并启动
docker compose -f deploy/sse-app/docker-compose.yml up -d --build

# 2) 查看状态/日志
docker compose -f deploy/sse-app/docker-compose.yml ps
docker compose -f deploy/sse-app/docker-compose.yml logs -f backend

# 3) 停止 / 完全清理
docker compose -f deploy/sse-app/docker-compose.yml down        # 停止
docker compose -f deploy/sse-app/docker-compose.yml down -v     # 连 sse-uploads 卷一起删
```

启动后：
- 后端 API：`http://127.0.0.1:8080`（前端 axios 仍直连该地址，CORS 已放行）
- 前端页面：`http://localhost:8090`

## 环境变量

未设置则用安全默认值（本地沙箱可直接 up）。要覆盖时复制 `.env.example` 为 `.env`：

```bash
cp deploy/sse-app/.env.example deploy/sse-app/.env
```

常用变量：

| 变量 | 说明 | 默认 |
|------|------|------|
| `SSE_LOG_HOST_DIR` | 后端日志文件挂载到的宿主机目录（Promtail 采集它） | `../../logs`（即“项目根/logs”） |
| `SSE_NEO4J_URI/USER/PASSWORD` | Neo4j 连接 | `bolt://host.docker.internal:7687` / neo4j/neo4j |
| `SSE_MYSQL_DSN` | MySQL DSN | `root:root@tcp(host.docker.internal:3306)/knowledge_graph?...` |
| `SSE_IMAGE_TAG` | 镜像 tag | latest |
| `GLM_API_KEY` / `BAIDU_CLIENT_ID` / `BAIDU_CLIENT_SECRET` | 大模型/检索密钥 | 空 |
| `SSE_LOG_LEVEL` | 日志级别 | info |

> 注意：`.env` 里填了密钥后不要提交到 git（已在 `.dockerignore` 思路下不入库，但确保别 `git add`）。

## 与日志观测栈衔接

后端容器把 JSON 日志写入 `SSE_LOG_HOST_DIR` 默认的宿主机「项目根/logs」目录，
与改造前原生运行写的是**同一目录**，所以 `deploy/observability` 的 Promtail → Loki → Grafana
无需任何改动，日志面板继续工作。

## 已知限制

- **PDF 上传 OCR**：`api/model_api/use_model.go` 通过 `exec.Command("python", .../ocr.py)` 调用
  PaddleOCR/Spire.PDF 做解析，本镜像不打包 python 运行环境（该功能当前宿主机也未装 `python`，
  原生运行同样不可用）。如需启用，请自行在 backend 镜像加入 python + requirements，或改走外部 OCR 服务。
- **数据卷**：PDF 上传目录持久化在命名卷 `sse-uploads`（挂 `/app/resource/doc`）。
