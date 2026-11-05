package embedding

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/cloudwego/eino-ext/components/embedding/ollama"
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

// Embed 调用Ollama embedding接口生成文本向量（REQ-006 迁移为 eino-ext Ollama Embedder，
// 底层走 /api/embed 新接口，返回 L2 归一化向量）。eino 返回 []float64，此处转回 []float32。
// 与迁移前 /api/embeddings 接口仅归一化差异（方向精确一致 cos=1.0）；Neo4j cosine
// score 与向量范数无关，REQ-004 相似度阈值体系零漂移（REQ-006 design 变更 #1）。
func Embed(text string) ([]float32, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 每次调用动态读取配置，支持运行时调整与测试覆盖
	emb, err := ollama.NewEmbedder(ctx, &ollama.EmbeddingConfig{
		BaseURL: getEnv("OLLAMA_URL", ollamaURL),
		Model:   getEnv("EMBED_MODEL", embedModel),
		Timeout: timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("构造Embedder失败: %w", err)
	}

	vectors, err := emb.EmbedStrings(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("调用embedding服务失败: %w", err)
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		return nil, fmt.Errorf("embedding结果为空")
	}

	vec := make([]float32, len(vectors[0]))
	for i, v := range vectors[0] {
		vec[i] = float32(v)
	}
	return vec, nil
}
