package embedding

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// 配置默认值（可通过环境变量覆盖：OLLAMA_URL / EMBED_MODEL）
var (
	// Ollama服务地址
	ollamaURL = "http://127.0.0.1:11434"
	// embedding模型名称（开源模型bge-m3）
	embedModel = "bge-m3"
	// 请求超时时间
	timeout = 15 * time.Second
)

// getEnv 读取环境变量，未设置时返回默认值
func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Embed 调用Ollama embedding接口生成文本向量
func Embed(text string) ([]float32, error) {
	// 每次调用动态读取配置，支持运行时调整与测试覆盖
	baseURL := getEnv("OLLAMA_URL", ollamaURL)
	model := getEnv("EMBED_MODEL", embedModel)

	// 构造请求数据
	payload := map[string]interface{}{
		"model":  model,
		"prompt": text,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("构造请求失败: %v", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", baseURL+"/api/embeddings", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 发送请求并获取响应
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用embedding服务失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding服务返回异常状态码: %d", resp.StatusCode)
	}

	// 解析响应
	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}
	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("embedding结果为空")
	}

	return result.Embedding, nil
}
