// ============================================================
// SSE 知识图谱服务 CI/CD 流水线
//
// 运行位置：deploy/jenkins 托管的 Jenkins（控制节点=Jenkins 容器）
// 运行方式：Jenkins 容器内已内置 docker CLI + compose，并通过挂载的
//           /var/run/docker.sock 驱动“宿主机”Docker，因此：
//           - 单元测试 / 静态检查  在 golang 官方镜像容器中执行
//           - 镜像构建 / 部署       由宿主机 docker daemon 执行
//
// 工作区：/Users/xieenping/work/kg-ci
//   该目录以“相同绝对路径”同时挂载进 Jenkins 容器，因此宿主机 Docker
//   （compose build 上下文、bind mount）看到的路径与 Jenkins 一致。
//
// 触发方式：Job 预置为每 2 分钟轮询 GitHub main（见 deploy/jenkins/jobs/...）
// ============================================================

pipeline {
    agent {
        node {
            label 'built-in'
            customWorkspace '/Users/xieenping/work/kg-ci'
        }
    }

    options {
        disableConcurrentBuilds()
        timestamps()
        timeout(time: 30, unit: 'MINUTES')
    }

    environment {
        // 与 Jenkins 容器内/宿主机保持一致的共享路径
        CI_ROOT            = '/Users/xieenping/work/kg-ci'
        COMPOSE_FILE       = '/Users/xieenping/work/kg-ci/deploy/sse-app/docker-compose.yml'
        COMPOSE_PROJECT    = 'sse'
        // 后端容器日志挂载到宿主机该目录，Promtail 继续采集（Grafana 面板不中断）
        SSE_LOG_HOST_DIR   = '/Users/xieenping/work/Knowledge-Graph/logs'
        // 镜像 tag：可在“Jenkinsfile 环境”或 job 参数中覆盖为构建号
        SSE_IMAGE_TAG      = 'latest'
    }

    stages {
        stage('检出') {
            steps {
                script {
                    // job SCM 配置了 CleanBeforeCheckout，Jenkins 在加载流水线时已把
                    // 目标 revision 完整克隆/检入工作区（见日志中“Checking out Revision”）。
                    // 这里不重复 checkout scm，避免对不稳定网络的二次依赖；仅在极少数
                    // 工作区缺失时兜底克隆一次。
                    def rev = sh(script: 'git rev-parse --short HEAD 2>/dev/null || echo NONE', returnStdout: true).trim()
                    if (rev == 'NONE') {
                        retry(3) { checkout scm }
                    }
                }
                sh 'echo "构建版本: $(git rev-parse --short HEAD) @ $(git log -1 --format=%cs)"'
            }
        }

        stage('静态检查与单元测试') {
            steps {
                sh """
                    docker run --rm \
                      -v ${CI_ROOT}:/src -w /src \
                      -e GOFLAGS=-mod=vendor -e GOTOOLCHAIN=local \
                      golang:1.23 sh -c 'go vet ./... && go test ./...'
                """
            }
        }

        stage('构建镜像') {
            steps {
                sh "docker compose -p ${COMPOSE_PROJECT} -f ${COMPOSE_FILE} build backend web"
            }
        }

        stage('部署') {
            steps {
                sh """
                    docker compose -p ${COMPOSE_PROJECT} -f ${COMPOSE_FILE} up -d --remove-orphans
                    docker image prune -f >/dev/null 2>&1 || true
                """
            }
        }

        stage('健康检查') {
            steps {
                sh """
                    ok=0
                    for i in \$(seq 1 20); do
                        code=\$(curl -s -o /dev/null -w '%{http_code}' http://host.docker.internal:8080/healthz || true)
                        if [ "\$code" = "200" ]; then echo 'backend /healthz -> 200 (就绪)'; ok=1; break; fi
                        sleep 3
                    done
                    docker compose -p ${COMPOSE_PROJECT} -f ${COMPOSE_FILE} ps
                    if [ "\$ok" != "1" ]; then echo '健康检查失败：backend /healthz 未在预期时间内返回 200' >&2; exit 1; fi
                """
            }
        }
    }

    post {
        always {
            script {
                echo "流水线结束：结果 ${currentBuild.result ?: 'SUCCESS'}"
            }
        }
    }
}
