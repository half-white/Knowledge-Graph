package router

import "SSE/api"

func (router RouterGroup) GraphRouter() {
	app := api.ApiGroupApp.GraphApi
	router.POST("model", app.GenGraph)
}
