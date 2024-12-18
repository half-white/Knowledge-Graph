package common

import (
	"SSE/global"
	"SSE/models"

	"gorm.io/gorm"
)

type Option struct {
	models.PageInfo
	Debug bool
}

// 查询Mysql里面的数据
func ComList[T any](model T, option Option) (list []T, count int64, err error) {

	DB := global.Mysql
	if option.Debug {
		DB = global.Mysql.Session(&gorm.Session{})
	}

	if option.Sort == "" {
		option.Sort = "created_at desc" //默认按照时间往前排
	}
	query := DB.Where(model)

	// 查询数量，可以给"id"列增加索引
	count = query.Select("id").Find(&list).RowsAffected
	// 这里的query会受到上面查询影响，需要手动复位
	query = DB.Where(model)
	offset := (option.Page - 1) * option.Limit
	if offset < 0 {
		offset = 0
	}
	err = query.Limit(option.Limit).Offset(offset).Order(option.Sort).Find(&list).Error

	return list, count, err
}
