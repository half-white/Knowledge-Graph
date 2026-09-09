// Package config 集中管理项目运行配置。
//
// 背景：项目里原本用一批散落的环境变量（GLM_API_KEY、SSE_NEO4J_URI、SSE_LOG_* …）
// 控制行为，部署改为 Docker 后本地调试常常漏配。本包把这些配置收拢到
// config/config.yml 一份文件里，作为“原来散落环境变量 / .env.example 内容”的
// 统一落地实现（config.example.yml 为入库模板），启动时统一读取并注入。
//
// 优先级：显式环境变量（Docker compose 注入 / shell export） > config/config.yml，
// 因此容器化部署里 compose 注入的值始终生效，不受本文件影响。
package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config 对应 config/config.yml 的结构。
// 各标量用 Raw 类型，避免 YAML 中 bool/数字/字符串 解析冲突。
type Config struct {
	Server ServerConfig `yaml:"server"`
	Neo4j  Neo4jConfig  `yaml:"neo4j"`
	MySQL  MySQLConfig  `yaml:"mysql"`
	Log    LogConfig    `yaml:"log"`
	LLM    LLMConfig    `yaml:"llm"`
	RAG    RAGConfig    `yaml:"rag"`
}

type ServerConfig struct {
	Addr Raw `yaml:"addr"`
}

type Neo4jConfig struct {
	URI      Raw `yaml:"uri"`
	User     Raw `yaml:"user"`
	Password Raw `yaml:"password"`
}

type MySQLConfig struct {
	DSN Raw `yaml:"dsn"`
}

type LogConfig struct {
	Level   Raw `yaml:"level"`
	Console Raw `yaml:"console"`
	File    Raw `yaml:"file"`
	Dir     Raw `yaml:"dir"`
	Keep    Raw `yaml:"keep"`
}

type LLMConfig struct {
	GLMAPIKey         Raw `yaml:"glm_api_key"`
	BaiduClientID     Raw `yaml:"baidu_client_id"`
	BaiduClientSecret Raw `yaml:"baidu_client_secret"`
}

type RAGConfig struct {
	Threshold Raw `yaml:"threshold"`
	TopK      Raw `yaml:"top_k"`
}

// Raw 保留 YAML 标量的原始文本（bool/数字/字符串统一按书写文本读取）。
type Raw string

func (r *Raw) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return nil
	}
	*r = Raw(node.Value)
	return nil
}

// Load 在 main 启动早期调用：
//  1. 读取 config/config.yml（可用环境变量 SSE_CONFIG 覆盖路径）；
//  2. 文件缺失或解析失败时静默跳过（全部走代码默认值 / Docker 注入变量）；
//  3. 把配置项写入对应环境变量——仅当该变量“尚未设置”时写入，
//     因此 Docker（compose 注入）或显式 export 的变量优先级更高。
func Load() {
	path := os.Getenv("SSE_CONFIG")
	if path == "" {
		path = "config/config.yml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return
	}
	cfg.apply()
}

// apply 把非空的配置项注入到项目各处读取的环境变量。
func (c *Config) apply() {
	setEnv("SSE_ADDR", string(c.Server.Addr))

	setEnv("SSE_NEO4J_URI", string(c.Neo4j.URI))
	setEnv("SSE_NEO4J_USER", string(c.Neo4j.User))
	setEnv("SSE_NEO4J_PASSWORD", string(c.Neo4j.Password))

	setEnv("SSE_MYSQL_DSN", string(c.MySQL.DSN))

	setEnv("SSE_LOG_LEVEL", string(c.Log.Level))
	setEnv("SSE_LOG_CONSOLE", string(c.Log.Console))
	setEnv("SSE_LOG_FILE", string(c.Log.File))
	setEnv("SSE_LOG_DIR", string(c.Log.Dir))
	setEnv("SSE_LOG_KEEP", string(c.Log.Keep))

	setEnv("GLM_API_KEY", string(c.LLM.GLMAPIKey))
	setEnv("BAIDU_CLIENT_ID", string(c.LLM.BaiduClientID))
	setEnv("BAIDU_CLIENT_SECRET", string(c.LLM.BaiduClientSecret))

	setEnv("RAG_THRESHOLD", string(c.RAG.Threshold))
	setEnv("RAG_TOP_K", string(c.RAG.TopK))
}

// setEnv 仅在没有显式环境变量时用配置文件补值。
func setEnv(key, value string) {
	if value == "" {
		return
	}
	if os.Getenv(key) == "" {
		_ = os.Setenv(key, value)
	}
}
