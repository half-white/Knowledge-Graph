package router

import "SSE/api"

func (router RouterGroup) ModelRouter() {
	app := api.ApiGroupApp.ModelApi
	router.POST("model", app.UseModel) // 输入文本创建图谱
	router.POST("upload", app.GetPdf)  // 上传文件创建图谱
}
