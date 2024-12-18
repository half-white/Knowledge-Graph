package models

// 保存图谱
type GraphModel struct {
	MODEL
	Title string `json:"title"` // 图谱标题
	UUID  string `json:"id"`    // uuid
}
