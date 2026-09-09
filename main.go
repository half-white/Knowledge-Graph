package main

import (
	"SSE/config"
	"SSE/database"
	"SSE/flag"
	"SSE/global"
	"SSE/logger"
	"SSE/router"
	"log/slog"
	"os"
)

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
