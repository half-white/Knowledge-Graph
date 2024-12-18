package flag

import (
	"SSE/global"
	"SSE/models"
	"fmt"
)

func Makemigrations() {
	var err error
	err = global.Mysql.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(
		&models.GraphModel{},
	)
	if err != nil {
		fmt.Println("[ error ] 生成数据库表结构失败")
		return
	}
	fmt.Println("[ success ] 生成数据库表结构成功")
}
