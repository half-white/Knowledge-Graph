package main

import (
	"SSE/config"
	"SSE/database"
	"SSE/flag"
	"SSE/global"
	"SSE/logger"
	"SSE/internal/router"
	"log/slog"
	"os"

	// swag 生成的接口文档（REQ-007）：docs/swagger 目录被 .gitignore 放行入库，
	// 匿名导入触发包内 init 注册文档，供 gin-swagger 托管 /swagger 文档页
	_ "SSE/docs/swagger"
)

// @Title        SSE 知识图谱服务
// @Version      1.0
// @Description  一键生成知识图谱服务：文本/PDF 上传 → LLM 三元组抽取 → Neo4j 图谱构建。
// @Description  统一响应格式 {code, data, msg}：code=0 成功，code=7 失败，HTTP 恒为 200。
// @Description  成功响应的 data 为 ECharts 图谱 JSON 的字符串形式（前端 JSON.parse 后渲染），
// @Description  msg 为三元组抽取文本明细。接口注释规范见 README「接口文档」章节。
// @Host         localhost:8080
func main() {
	// 加载项目配置（config/config.yml，须在任何日志/数据库/密钥读取之前调用）
	config.Load()

	//初始化日志系统（必须在任何日志输出之前调用）
	logger.Init()

	//连接Neo4j数据库
	global.DB = database.Connect_Neo4j()
	global.Mysql = database.Connect_Mysql()

	//命令行参数绑定
	option := flag.Parse()
	if flag.IsWebStop(option) {
		flag.UseOption(option)
		return
	}

	//路由配置
	router := router.InitRouter()

	// 监听地址可用环境变量 SSE_ADDR 覆盖（默认 127.0.0.1:8080；
	// Docker 容器内需监听全部网卡，例如 SSE_ADDR=:8080）
	addr := os.Getenv("SSE_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	slog.Info("一键生成知识图谱服务运行", "addr", addr)
	if err := router.Run(addr); err != nil {
		slog.Error("服务启动失败", "error", err)
	}
}
