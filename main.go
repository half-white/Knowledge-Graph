package main

import (
	"SSE/database"
	"SSE/flag"
	"SSE/global"
	"SSE/logger"
	"SSE/router"
	"log/slog"
)

func main() {
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

	addr := "127.0.0.1:8080"
	slog.Info("一键生成知识图谱服务运行", "addr", addr)
	if err := router.Run(addr); err != nil {
		slog.Error("服务启动失败", "error", err)
	}
}
