package database

import (
	"log/slog"
	"os"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Connect_Neo4j() neo4j.Driver {
	// 连接Neo4j系统服务（v5驱动，bolt://默认明文连接）
	driver, err := neo4j.NewDriver("bolt://localhost:7687", neo4j.BasicAuth("neo4j", "neo4j", ""))
	if err != nil {
		slog.Error("创建 Neo4j 驱动失败", "error", err)
		os.Exit(1)
	}

	// 验证连接（v5驱动 Driver.VerifyConnectivity）
	if err := driver.VerifyConnectivity(); err != nil {
		slog.Error("Neo4j 连接失败", "endpoint", "bolt://localhost:7687", "error", err)
		os.Exit(1)
	}
	slog.Info("Neo4j 连接成功", "endpoint", "bolt://localhost:7687")

	return driver
}

func Connect_Mysql() *gorm.DB {
	dsn := "root:root@tcp(localhost:3306)/knowledge_graph?charset=utf8mb4&parseTime=True&loc=Local"

	db, err := gorm.Open(mysql.Open(dsn))
	if err != nil {
		slog.Error("MySQL 连接失败", "dsn", dsn, "error", err)
		os.Exit(1)
	}
	slog.Info("MySQL 连接成功", "addr", "localhost:3306", "database", "knowledge_graph")

	sqlDB, _ := db.DB()
	sqlDB.SetMaxIdleConns(10)               //最大空闲连接数
	sqlDB.SetMaxOpenConns(100)              //最多可容纳
	sqlDB.SetConnMaxLifetime(time.Hour * 4) //连接最大复用时间，不能超过mysql的wait_timeout
	return db
}
