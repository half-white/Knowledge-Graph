package flag

import (
	"SSE/global"
	"SSE/models"
	"log/slog"
)

func Makemigrations() {
	var err error
	err = global.Mysql.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(
		&models.GraphModel{},
	)
	if err != nil {
		slog.Error("生成数据库表结构失败", "error", err)
		return
	}
	slog.Info("生成数据库表结构成功")
}
