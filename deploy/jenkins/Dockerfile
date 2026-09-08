# ============================================================
# Jenkins 控制节点镜像
# 在官方 jenkins 镜像之上补充：
#   docker CLI + docker compose v2 插件（由 docker:28-cli 镜像直接拷贝，
#   版本随官方镜像固定，无需在构建时下载安装）
# 配合宿主机挂载的 /var/run/docker.sock，流水线可直接驱动宿主机 Docker，
# 完成镜像构建（golang/node）与 docker compose 滚动部署。
#
# 说明：官方 jenkins 镜像已内置 git / curl / java，无需额外安装。
# ============================================================

# docker CLI / compose 插件来源（含对应 CLI 的 compose 插件）
FROM docker:28-cli AS dockercli

# 基础镜像固定到本机官方镜像 digest（jenkins/jenkins:latest 在本机的实际版本），
# 避免用浮动 latest 导致重建镜像时 Jenkins 意外升级
FROM jenkins/jenkins@sha256:7c6778146db313c8a509fec4ecbe0681efaa00f60e550460caf3b2b5d66e9767

USER root

# docker CLI + compose 插件（compose/buildx 位于 libexec 目录）
COPY --from=dockercli /usr/local/bin/docker /usr/local/bin/docker
COPY --from=dockercli /usr/local/libexec/docker/cli-plugins /usr/local/libexec/docker/cli-plugins

USER jenkins
