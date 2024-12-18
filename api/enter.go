package api

import (
	"SSE/api/graph_api"
	"SSE/api/model_api"
)

type ApiGroup struct {
	ModelApi model_api.ModelApi
	GraphApi graph_api.GraphApi
}

var ApiGroupApp = new(ApiGroup)
