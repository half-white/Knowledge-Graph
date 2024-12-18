package main

import (
	"SSE/database"
	"SSE/flag"
	"SSE/global"
	"SSE/router"
	"fmt"
)

func main() {
	//读取配置文件

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
	fmt.Println("\"一键生成知识图谱\"服务运行在：", addr)
	router.Run(addr)
}
