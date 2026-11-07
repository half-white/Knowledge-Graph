# SSE 知识图谱（Knowledge-Graph）

一键生成知识图谱服务：上传文档 → 大模型抽取实体-关系三元组 → 存入 Neo4j；
配合 RAG 向量检索增强抽取质量。Go(gin) 后端 + Vue3 前端，Docker 化部署。

## 目录结构

```
main.go               程序入口（加载配置 → 日志 → 数据库 → 路由）
config/               启动配置（config.yml 本地私有已忽略；config.example.yml 入库模板）
router/               路由注册（model_router / graph_router / healthz）
api/                  控制器层（model_api 大模型与OCR；graph_api 图谱生成）
common/               通用业务查询（MySQL 数据的通用增删查）
models/               数据库模型与请求/响应结构体（GORM）
database/             Neo4j / MySQL 连接
global/               全局变量（global.DB / global.Mysql）
response/             HTTP 响应封装（统一返回格式与错误码）
logger/               日志系统（控制台 + 按天轮转文件）
utils/                工具库（rag/ 向量检索、embedding/、pdf/word 解析、ocr.py）
frontend/             Vue3 + Vite 前端项目
deploy/               部署编排（sse-app 应用栈 / observability 日志观测 / jenkins）
resource/doc/         运行时 PDF 上传目录（容器内挂载为持久卷）
vendor/               本地依赖快照（已不入库；依赖以 go.mod/go.sum 为准，CI/镜像构建经 go mod download 拉取）
triplets.csv          示例三元组数据（头实体,关系,尾实体），仅供演示参考，代码不读取
logs/                 运行时日志输出（已 gitignore）
docs/swagger/         swag 生成的接口文档（docs.go/swagger.json，入库；注释变更后需重新生成）
docs/defspec/         DefSpec 需求工作流文档（已 gitignore）
```

## 快速开始

```bash
# 本地运行（先复制并填写配置）
cp config/config.example.yml config/config.yml
go run .                      # 默认监听 127.0.0.1:8080

# Docker 部署（后端 8080 / 前端 8090）
docker compose -f deploy/sse-app/docker-compose.yml up -d --build
```

更多部署与 CI/CD 说明见 `deploy/README-CICD.md` 与 `deploy/sse-app/README.md`。

## 接口文档（Swagger）

接口文档由 [swag](https://github.com/swaggo/swag) 注释驱动生成，源码即文档，无外部平台依赖：

- **注释位置**：全局信息在 `main.go`（@Title/@Description 等）；接口信息在各 handler
  上方（`api/` 包）。`@Router` 必须与 `router/` 实际注册的路由一致——只注释真实挂载的接口
- **开启文档页**（默认关闭，生产不暴露接口信息）：
  1. `config/config.yml` 中设置 `server.swagger: "true"`（或环境变量 `SSE_SWAGGER=true`，
     取值语义与日志开关一致：true/1/yes/on 开，false/0/no/off 关）
  2. 启动服务后访问 `http://127.0.0.1:8080/swagger/index.html`
- **修改注释后重新生成**（开发机装一次 CLI，版本与 go.mod 中 `swaggo/swag` 一致）：
  ```bash
  go install github.com/swaggo/swag/cmd/swag@v1.16.6
  swag init -g main.go -o docs/swagger --exclude vendor
  ```
- 生成物 `docs/swagger/` 已入库，CI 与其他机器无需安装 CLI
