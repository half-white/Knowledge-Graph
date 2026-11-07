package router

import "SSE/internal/api"

func (router RouterGroup) GraphRouter() {
	app := api.ApiGroupApp.GraphApi
	router.POST("model", app.GenGraph)
}
