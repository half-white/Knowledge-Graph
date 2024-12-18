package models

import "time"

type MODEL struct {
	ID        uint      `gorm:"primarykey" json:"id" structs:"-"` //主键
	CreatedAt time.Time `json:"created_at" structs:"-"`           //创建时间
	UpdatedAt time.Time `json:"-" structs:"-"`                    //更新时间
}

type RemoveRequest struct {
	UUID string `json:"uuid" binding:"required"`
}

type QueryRequest struct {
	UUID string `json:"uuid" binding:"required"`
}

type PageInfo struct {
	Page  int    `form:"page"`  // 查询哪一页
	Key   string `form:"key"`   // 查询的关键词
	Limit int    `form:"limit"` // 每一页的数量
	Sort  string `form:"sort"`  // 排序方式
}
