package router

import (
	"github.com/gin-gonic/gin"
)

type RouterGroup struct {
	*gin.RouterGroup
}

func InitRouter() *gin.Engine {
	gin.SetMode("release")

	//启动服务
	router := gin.Default()

	// // 配置静态文件服务(前端的静态资源)
	// router.Static("/static", "./static")

	// // 配置HTML模板文件夹
	// router.LoadHTMLGlob("templates/*")

	//设置跨域请求,使用中间件处理跨域问题
	router.Use(CORSMiddleware())

	// // 路由:渲染首页
	// router.GET("/", func(c *gin.Context) {
	// 	c.HTML(http.StatusOK, "index.html", gin.H{
	// 		"title": "首页",
	// 	})
	// })

	//设置路由组
	apiRouterGroup := router.Group("api")
	routerGroupApp := RouterGroup{apiRouterGroup}

	//系统配置api
	routerGroupApp.ModelRouter()

	return router
}

// CORSMiddleware 中间件处理跨域问题
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
