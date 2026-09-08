# SSE 知识图谱 — Docker 化 + Jenkins CI/CD 部署总览

本仓库现包含两部分可独立使用的部署体系：

| 目录 | 作用 |
|------|------|
| `deploy/sse-app/`   | **应用栈**：后端(Go) + 前端(Vue) 容器化，docker compose 一键托管 |
| `deploy/jenkins/`   | **CI/CD 控制节点**：Jenkins 容器 + 流水线任务 + 数据卷迁移脚本 |
| `deploy/observability/` | 日志观测（Loki + Promtail + Grafana），**无需改动** |
| 仓库根 `Jenkinsfile` | 流水线定义（检出→单测→构建镜像→部署→健康检查） |

## 快速路径

```bash
# A. 仅用 Docker 部署应用（不依赖 Jenkins）
docker compose -f deploy/sse-app/docker-compose.yml up -d --build

# B. 启用 Jenkins CI/CD（首次初始化）
bash deploy/jenkins/setup-jenkins.sh
# 之后每次 push 到 GitHub main，2 分钟内自动 构建→测试→部署
```

## 流水线内容（Jenkinsfile）

```
检出(GitHub main, 工作区 /Users/xieenping/work/kg-ci)
  → 静态检查与单元测试(golang:1.23 容器: go vet + go test)
  → 构建镜像(docker compose build: sse/backend, sse/web)
  → 部署(docker compose -p sse up -d)
  → 健康检查(HTTP 探测 http://host.docker.internal:8080)
```

## 架构要点

- **Jenkins 在容器里，构建/部署用宿主机 docker**：Jenkins 容器内装 docker CLI +
  compose 插件，挂载 `/var/run/docker.sock`，所有镜像构建与 `docker compose` 操作都由
  宿主机 daemon 执行（Jenkins 无需安装 Go/Node）。
- **共享工作区同路径挂载**：`/Users/xieenping/work/kg-ci` 在 Jenkins 容器与宿主机是同一
  绝对路径，因此 docker 构建上下文与 compose 部署文件对两端“可见”。
- **数据保留**：旧 Jenkins 的账号/插件/jobs 通过 setup 脚本迁入命名卷 `jenkins_home`；
  端口不变 `localhost:55000`。
- **代码可移植**：`database/connect.go`、`main.go` 把 Neo4j/MySQL 地址与监听地址改为
  环境变量可覆盖，默认值保持改造前一致，宿主机原生 `go build && ./SSE` 照常可用。
- **日志衔接**：容器后端写「项目根/logs」与原生一致，Promtail/Loki/Grafana 面板照常工作。

## 涉及本机路径清单

| 路径 | 作用 |
|------|------|
| `/var/lib/docker/volumes/jenkins_home` | Jenkins 数据卷（映射原匿名卷迁移而来） |
| `/Users/xieenping/work/kg-ci` | CI 工作区（每次流水线删除重建，可 sudo 清理） |
| `/Users/xieenping/work/Knowledge-Graph/logs` | 后端日志（Promtail 采集） |

## 注意

- 本机 Jenkins 无公网入口，无法接 GitHub webhook，采用 **SCM 轮询（2 分钟）**。
- Git 仓库是 public，job 用 HTTPS 检出免凭据；若将来转 private，需在 Jenkins 配凭据并改
  `deploy/jenkins/jobs/SSE-KnowledgeGraph/config.xml`。
- 数据库账号密码、模型 API Key 均为本地沙箱默认值/空值；对外发布前务必通过
  `.env` / Jenkins 凭据收敛密钥。
