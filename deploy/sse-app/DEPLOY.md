# VPS 公网部署指南

本文档描述如何把本项目部署到一台云服务器（VPS），通过自己的域名对外提供访问。

## 架构总览

```
用户浏览器
   │  https://your-domain.com
   ▼
VPS :80（sse-web 容器，nginx）
   ├── /            → 前端静态资源（frontend/ 构建产物）
   └── /api/        → 反向代理到 backend 容器 :8080（Docker 内网）
                          │
                          ▼
                    Neo4j / MySQL（仅限 Docker 内网或 127.0.0.1）
```

前端代码统一使用相对路径 `/api/...`，开发环境由 Vite 代理、生产环境由 nginx 代理，
前后端同源，无 CORS 问题。

## 前提条件

- 一台 VPS（1核2G 起步可跑，推荐 2核4G），已安装 Docker 和 Docker Compose 插件
- 一个域名，DNS A 记录解析到 VPS 公网 IP
- 项目代码已上传到 VPS（git clone 或 scp）

## 部署步骤

### 1. 配置环境变量

```bash
cd deploy/sse-app
cp .env.example .env
vi .env    # 填入 GLM_API_KEY、数据库密码等
```

### 2. 构建并启动

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

首次构建需要下载依赖，约 3-5 分钟。之后浏览器访问 `http://your-domain.com` 即可使用。

### 3. 配置 HTTPS（强烈建议）

推荐用 Caddy 做外层网关（自动申请续期证书），在 VPS 上：

```bash
# /etc/caddy/Caddyfile
your-domain.com {
    reverse_proxy 127.0.0.1:80
}
```

`systemctl enable --now caddy` 后自动完成证书申请与续期。
（若云厂商安全组/防火墙放行了 80 和 443 端口，在控制台里先放行。）

### 4. 安全检查清单（上线前逐项确认）

- [ ] Neo4j(7474/7687)、MySQL(3306)、Grafana(3000)、Loki(3100)、Jenkins
      **均未对公网发布**：`docker ps` 查看 PORTS 列，公网入口只允许 80/443
- [ ] 数据库默认密码（neo4j/neo4j、root/root）已修改，通过 `.env` 注入
- [ ] 后端当前无用户鉴权，公网可匿名调用。建议在 nginx/Caddy 层加一层
      Basic Auth 或在业务上自行增加认证后再开放
- [ ] GLM_API_KEY 等密钥只存在 `.env`（已在 .gitignore 中），不进代码仓库
- [ ] 服务器 SSH 改用密钥登录、禁用密码

## 常用运维命令

```bash
# 查看状态 / 日志
docker compose -f docker-compose.yml -f docker-compose.prod.yml ps
docker logs -f sse-backend

# 更新代码后重新发布
git pull
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build

# 停止 / 重启
docker compose -f docker-compose.yml -f docker-compose.prod.yml down
```

## 数据备份

图谱和业务数据分别在 Neo4j 与 MySQL 的 Docker 卷中，定期备份：

```bash
docker exec knowledge-graph-neo4j neo4j-admin database dump neo4j --to-stdout > neo4j-$(date +%F).dump
docker exec kg-mysql mariadb-dump -uroot -p"密码" knowledge_graph > mysql-$(date +%F).sql
```
