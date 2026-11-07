package api

import (
	"SSE/internal/api/graph_api"
	"SSE/internal/api/model_api"
)

type ApiGroup struct {
	ModelApi model_api.ModelApi
	GraphApi graph_api.GraphApi
}

var ApiGroupApp = new(ApiGroup)
