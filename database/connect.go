package database

import (
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// env 读取环境变量，未设置时返回默认值
func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Connect_Neo4j 连接 Neo4j（v5 驱动，bolt:// 默认明文连接）。
//
// 连接参数可通过环境变量覆盖（默认值=宿主机原生运行时的地址）：
//   - SSE_NEO4J_URI       默认 bolt://localhost:7687（容器内部署时指向 host.docker.internal）
//   - SSE_NEO4J_USER      默认 neo4j
//   - SSE_NEO4J_PASSWORD  默认 neo4j
func Connect_Neo4j() neo4j.Driver {
	uri := env("SSE_NEO4J_URI", "bolt://localhost:7687")
	user := env("SSE_NEO4J_USER", "neo4j")
	password := env("SSE_NEO4J_PASSWORD", "neo4j")

	driver, err := neo4j.NewDriver(uri, neo4j.BasicAuth(user, password, ""))
	if err != nil {
		slog.Error("创建 Neo4j 驱动失败", "error", err)
		os.Exit(1)
	}

	// 验证连接（v5驱动 Driver.VerifyConnectivity）
	if err := driver.VerifyConnectivity(); err != nil {
		slog.Error("Neo4j 连接失败", "endpoint", uri, "error", err)
		os.Exit(1)
	}
	slog.Info("Neo4j 连接成功", "endpoint", uri)

	return driver
}

// mysqlAddr 从 DSN 中截取 tcp(host:port) 部分用于日志展示（不含账号密码）
func mysqlAddr(dsn string) string {
	const prefix = "tcp("
	i := strings.Index(dsn, prefix)
	if i < 0 {
		return dsn
	}
	rest := dsn[i+len(prefix):]
	if j := strings.Index(rest, ")"); j >= 0 {
		return rest[:j]
	}
	return rest
}

// Connect_Mysql 连接 MySQL/MariaDB。
//
// DSN 可通过环境变量 SSE_MYSQL_DSN 覆盖（默认值=宿主机原生运行时的地址），
// 例如容器内部署时指向：root:root@tcp(host.docker.internal:3306)/knowledge_graph?...
func Connect_Mysql() *gorm.DB {
	dsn := env("SSE_MYSQL_DSN", "root:root@tcp(localhost:3306)/knowledge_graph?charset=utf8mb4&parseTime=True&loc=Local")

	db, err := gorm.Open(mysql.Open(dsn))
	if err != nil {
		slog.Error("MySQL 连接失败", "dsn", dsn, "error", err)
		os.Exit(1)
	}
	slog.Info("MySQL 连接成功", "addr", mysqlAddr(dsn), "database", "knowledge_graph")

	sqlDB, _ := db.DB()
	sqlDB.SetMaxIdleConns(10)               //最大空闲连接数
	sqlDB.SetMaxOpenConns(100)              //最多可容纳
	sqlDB.SetConnMaxLifetime(time.Hour * 4) //连接最大复用时间，不能超过mysql的wait_timeout
	return db
}
