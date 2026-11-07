package router

import (
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type RouterGroup struct {
	*gin.RouterGroup
}

func InitRouter() *gin.Engine {
	gin.SetMode("release")

	//启动服务（gin.New 不附带默认中间件，改用自实现的结构化访问日志）
	router := gin.New()
	router.Use(gin.Recovery())

	// // 配置静态文件服务(前端的静态资源)
	// router.Static("/static", "./static")

	// // 配置HTML模板文件夹
	// router.LoadHTMLGlob("templates/*")

	//设置跨域请求,使用中间件处理跨域问题
	router.Use(CORSMiddleware())

	// 结构化访问日志（输出到全局 slog）
	router.Use(RequestLogger())

	// 健康检查端点（供容器 healthcheck / CI 探测；返回 200）
	router.GET("/healthz", healthz)

	// swagger 文档页：默认关闭（生产不暴露接口信息）；config.yml server.swagger
	// 或环境变量 SSE_SWAGGER 控制开启，取值语义与 logger.envBool 一致（REQ-007）
	if envBool("SSE_SWAGGER", false) {
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

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

// RequestLogger 记录每个 HTTP 请求的结构化访问日志（方法/路径/状态码/耗时/客户端IP）
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		// 在响应后统一打印（含真实状态码）
		slog.Info("http_request",
			"method", c.Request.Method,
			"path", path,
			"query", query,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
			"errors", c.Errors.String(),
		)
	}
}

// healthz 存活探针：供容器 healthcheck / CI 探测，恒返回 200 与文本 ok
// @Summary      健康检查
// @Description  存活探针，供容器 healthcheck / CI 探测，恒返回 HTTP 200 与文本 ok
// @Tags         系统
// @Produce      plain
// @Success      200 {string} string "ok"
// @Router       /healthz [get]
func healthz(c *gin.Context) {
	c.String(200, "ok")
}

// envBool 读取布尔环境变量：true/1/yes/on 为真，false/0/no/off 为假，
// 未设置或无法识别取默认值。与 logger.envBool 语义保持一致（REQ-007）。
func envBool(key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	}
	return def
}
