#!/usr/bin/env bash
# ============================================================
# SSE 应用栈启动/更新脚本（防呆版）
#
# 背景：docker compose 读取 .env 的位置是“执行命令时所在目录”，
# 而不是 compose 文件所在目录。从仓库根目录执行
#   docker compose -f deploy/sse-app/docker-compose.yml up -d
# 会导致 .env 读不到，GLM_API_KEY 等变量静默变空。
#
# 本脚本固定先切到本文件所在目录再执行 compose，避免该问题。
#
# 用法：
#   ./up.sh                # 启动/更新全部服务（含重建 env 有变化的容器）
#   ./up.sh backend        # 只启动/更新 backend
#   ./up.sh build backend  # 先构建再启动
# ============================================================
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
exec docker compose -p sse -f docker-compose.yml up -d "$@"
