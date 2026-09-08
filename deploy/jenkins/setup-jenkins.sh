#!/usr/bin/env bash
# ============================================================
# 一次性初始化：把现有“裸 docker run 的 Jenkins”平滑迁移为
# compose 托管的 Jenkins（deploy/jenkins/docker-compose.yml）
#
# 做的事情（全部可重复执行，幂等）：
#   1. 找到旧容器 jenkins 的匿名数据卷，把内容复制到命名卷 jenkins_home
#      （保留已有账号 / 插件 / 任务 / InkTune 等所有数据）
#   2. 删除旧容器（保留数据卷），避免与 compose 的 container_name 冲突
#   3. 预置 CI 流水线任务 jobs/SSE-KnowledgeGraph
#   4. 创建 CI 共享工作区 /Users/xieenping/work/kg-ci（与宿主机同路径挂载）
#   5. 构建自定义 Jenkins 镜像并 docker compose up -d
#
# 用法：  bash deploy/jenkins/setup-jenkins.sh
# ============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yml"

OLD_CONTAINER="jenkins"
VOL_NAME="jenkins_home"
WS_DIR="/Users/xieenping/work/kg-ci"          # 必须与 compose 挂载路径一致
JOB_NAME="SSE-KnowledgeGraph"
SEED_XML="${SCRIPT_DIR}/jobs/${JOB_NAME}/config.xml"

log() { echo -e "\033[1;34m[setup]\033[0m $*"; }

# ---------- 1. 定位旧数据卷 ----------
OLD_VOL=""
if docker ps -a --format '{{.Names}}' | grep -qx "${OLD_CONTAINER}"; then
  COMPOSE_PROJECT="$(docker inspect -f '{{ index .Config.Labels "com.docker.compose.project" }}' "${OLD_CONTAINER}" 2>/dev/null || true)"
  if [ "${COMPOSE_PROJECT}" = "sse-jenkins" ]; then
    log "已检测到 compose 托管的 Jenkins 容器，跳过旧卷迁移。"
  else
    OLD_VOL="$(docker inspect -f '{{ range .Mounts }}{{ if eq .Destination "/var/jenkins_home" }}{{ .Name }}{{ end }}{{ end }}' "${OLD_CONTAINER}")"
    log "旧容器数据卷: ${OLD_VOL:-<none>}"
  fi
else
  log "未发现旧容器 ${OLD_CONTAINER}，跳过迁移（若已有命名卷则复用）。"
fi

# ---------- 2. 创建命名卷并迁移 ----------
docker volume create "${VOL_NAME}" >/dev/null 2>&1 || true

volume_is_empty() {
  ! docker run --rm -v "${VOL_NAME}":/v alpine sh -c 'test -n "$(ls -A /v)"' 2>/dev/null
}

if [ -n "${OLD_VOL}" ] && [ "${OLD_VOL}" != "${VOL_NAME}" ] && volume_is_empty; then
  log "迁移 ${OLD_VOL} -> ${VOL_NAME} ..."
  docker run --rm \
    -v "${OLD_VOL}":/from:ro \
    -v "${VOL_NAME}":/to \
    alpine sh -c 'cp -a /from/. /to/ && chown -R 1000:1000 /to'
  log "数据迁移完成（含账号/插件/jobs）。"
fi

# ---------- 3. 删除旧容器（仅非 compose 管理时） ----------
if docker ps -a --format '{{.Names}}' | grep -qx "${OLD_CONTAINER}"; then
  COMPOSE_PROJECT="$(docker inspect -f '{{ index .Config.Labels "com.docker.compose.project" }}' "${OLD_CONTAINER}" 2>/dev/null || true)"
  if [ "${COMPOSE_PROJECT}" = "sse-jenkins" ]; then
    log "容器已是 compose 管理（project=sse-jenkins），保留，直接 up 复用。"
  else
    log "删除旧容器 ${OLD_CONTAINER}（数据已入命名卷，不受影响）..."
    docker rm -f "${OLD_CONTAINER}" >/dev/null
  fi
fi

# ---------- 4. 预置 CI 流水线任务 ----------
if [ -n "${SEED_XML}" ] && [ -f "${SEED_XML}" ]; then
  if docker run --rm -v "${VOL_NAME}":/jh alpine test -f "/jh/jobs/${JOB_NAME}/config.xml" 2>/dev/null; then
    log "Job ${JOB_NAME} 已存在，跳过预置。"
  else
    log "预置流水线任务 ${JOB_NAME} ..."
    docker run --rm \
      -v "${VOL_NAME}":/jh \
      -v "${SEED_XML}":/seed/config.xml:ro \
      alpine sh -c "mkdir -p /jh/jobs/${JOB_NAME} && cp /seed/config.xml /jh/jobs/${JOB_NAME}/config.xml && chown -R 1000:1000 /jh/jobs/${JOB_NAME}"
    log "Job 预置完成。"
  fi
else
  log "警告：未找到 ${SEED_XML}，跳过 Job 预置（可到 UI 手工创建 Pipeline Job）。"
fi

# ---------- 5. CI 共享工作区（宿主同路径挂载给 docker daemon 使用） ----------
mkdir -p "${WS_DIR}"
chmod 777 "${WS_DIR}"   # 容器内 jenkins(uid1000) 需要写入；清理时 sudo rm -rf 即可
log "CI 工作区就绪: ${WS_DIR}"

# ---------- 6. 构建并启动 ----------
log "构建 Jenkins 镜像并启动..."
docker compose -f "${COMPOSE_FILE}" up -d --build

log "完成。等待启动..."
log "  UI:    http://localhost:55000  （沿用原端口与账号）"
log "  Job:   SSE-KnowledgeGraph"
log "  日志:  docker compose -f deploy/jenkins/docker-compose.yml logs -f"
