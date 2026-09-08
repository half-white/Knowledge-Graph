# Jenkins（Docker 托管）— SSE 项目 CI/CD

本目录把原来用 `docker run` 临时启动的 Jenkins，升级为 `docker compose` 管理、带
**docker CLI + compose 插件**、并**保留原有账号/插件/任务数据**的 CI/CD 控制节点。

## 拓扑

```
GitHub(public) --poll 2min--> Jenkins(:55000)
                                 │  内置 docker CLI + compose 插件
                                 │  /var/run/docker.sock (宿主 docker daemon)
                                 ▼
   golang:1.23 容器跑 go vet/test ──┐
   宿主 docker: compose build/up ───┴→ sse-backend / sse-web 容器
```

## 目录

```
deploy/jenkins/
├── docker-compose.yml          # Jenkins 服务编排（端口 55000/55001）
├── Dockerfile                  # 定制镜像：jenkins + docker CLI + compose 插件 + curl
├── setup-jenkins.sh            # 【首次】一键初始化：迁移数据卷/预置 job/重建容器
├── jobs/SSE-KnowledgeGraph/
│   └── config.xml              # 预置流水线 job（SCM=GitHub main，2 分钟轮询）
└── README.md
```

## 首次初始化（只跑一次）

```bash
bash deploy/jenkins/setup-jenkins.sh
```

脚本会：
1. 找到旧容器 `jenkins` 的匿名数据卷，把内容复制进命名卷 `jenkins_home`（账号/插件/jobs 全保留）；
2. 删除旧容器；
3. 写入预置任务 `SSE-KnowledgeGraph`（GitHub HTTPS 检出 main，`scriptPath=Jenkinsfile`，
   每 2 分钟 SCM 轮询自动触发）；
4. 创建 CI 共享工作区 `/Users/xieenping/work/kg-ci`（chmod 777）；
5. `docker compose up -d --build`。

## 日常操作

```bash
# 启动 / 停止 / 查看
docker compose -f deploy/jenkins/docker-compose.yml up -d
docker compose -f deploy/jenkins/docker-compose.yml down
docker compose -f deploy/jenkins/docker-compose.yml logs -f

# UI
open http://localhost:55000      # 沿用你原来的账号登录
```

## 为什么这么设计

| 设计点 | 原因 |
|--------|------|
| 命名卷 `jenkins_home`（external） | 替换“匿名卷”，数据可控、可备份 |
| 固定基础镜像 digest | 避免浮动 `latest` 在重建时把 Jenkins 升级出意外（升级向导会阻塞启动） |
| 容器内装 docker CLI + 挂 docker.sock | “Docker outside of Docker”：流水线驱动宿主机 docker daemon 构建/部署 |
| `group_add: ["0"]` | 本机容器视角下 socket 属主 `root:root 660`，给 jenkins 用户补 root 组即可读写，容器不必整机 root |
| 共享工作区同路径挂载 `/Users/xieenping/work/kg-ci` | 让宿主 docker daemon 能看到与 Jenkins 一致的构建上下文/部署文件路径 |
| 预置 job 走 SCM 轮询 | 本地 Jenkins 无法接收 GitHub webhook（无公网入口），2 分钟轮询足够个人开发节奏 |

## 工作区与清理

- 流水线每次在 `/Users/xieenping/work/kg-ci` 从零克隆 main（`deleteDir()`），docker build 上下文即该目录。
- 该目录由容器内 jenkins(uid 1000) 写文件，宿主机上清理请用：
  ```bash
  sudo rm -rf /Users/xieenping/work/kg-ci
  ```
  删掉后下次构建会自动重建；不影响正在运行的容器（镜像已构建完成）。

## 回滚 / 手动部署

部署即对 compose 项目 `sse` 执行 `up -d`。想手动回滚到上一个镜像，可在
`deploy/sse-app` 构建并 `up`，或直接针对容器操作：

```bash
docker compose -f deploy/sse-app/docker-compose.yml up -d --build   # 从源码重建
```
