package database

import (
	"fmt"
	"log"
	"time"

	"github.com/neo4j/neo4j-go-driver/neo4j"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Connect_Neo4j() neo4j.Driver {
	// 连接Neo4j系统服务
	driver, err := neo4j.NewDriver("bolt://localhost:7687", neo4j.BasicAuth("neo4j", "neo4j", ""), func(c *neo4j.Config) {
		c.Encrypted = false
	})
	if err != nil {
		log.Fatalf("Failed to create Neo4j driver:%v", err)
	} else {
		fmt.Println("Connecting Neo4j success")
	}

	return driver
}

func Connect_Mysql() *gorm.DB {
	dsn := "root:root@tcp(localhost:3306)/knowledge_graph?charset=utf8mb4&parseTime=True&loc=Local"

	db, err := gorm.Open(mysql.Open(dsn))
	if err != nil {
		log.Fatalf(fmt.Sprintf("[%s]mysql连接失败", dsn))
		panic(err)
	} else {
		fmt.Println("MySQL连接成功")
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxIdleConns(10)               //最大空闲连接数
	sqlDB.SetMaxOpenConns(100)              //最多可容纳
	sqlDB.SetConnMaxLifetime(time.Hour * 4) //连接最大复用时间，不能超过mysql的wait_timeout
	return db
}
